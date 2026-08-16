package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	gcppubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/runtimeconfig"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreStateCrossesServiceProcessesAndSurvivesRestart(t *testing.T) {
	firestoreHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	pubsubHost := os.Getenv("PUBSUB_EMULATOR_HOST")
	if firestoreHost == "" || pubsubHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST and PUBSUB_EMULATOR_HOST are not both set")
	}
	if runtime.GOOS == "windows" {
		t.Skip("process signal assertions require POSIX signals")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	repositoryRoot := filepath.Clean("..")
	binaryDirectory := t.TempDir()
	lostBinary := filepath.Join(binaryDirectory, "lostpet-service")
	webBinary := filepath.Join(binaryDirectory, "web-frontend")
	buildTestBinary(t, ctx, repositoryRoot, lostBinary, "./cmd/lostpet-service")
	buildTestBinary(t, ctx, repositoryRoot, webBinary, "./cmd/web-frontend")

	projectID := "petspotr-process-contract"
	pubsubClient, err := gcppubsub.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("create Pub/Sub emulator client: %v", err)
	}
	t.Cleanup(func() { _ = pubsubClient.Close() })
	lostTopic, err := pubsubClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: fmt.Sprintf("projects/%s/topics/lostPet", projectID),
	})
	if err != nil {
		t.Fatalf("create lostPet emulator topic: %v", err)
	}
	t.Cleanup(func() {
		_ = pubsubClient.TopicAdminClient.DeleteTopic(context.Background(), &pubsubpb.DeleteTopicRequest{Topic: lostTopic.GetName()})
	})
	petID := fmt.Sprintf("lost-process-%d", time.Now().UnixNano())
	stateRuntime, err := runtimeconfig.NewStateRuntime(ctx, runtimeconfig.StateConfig{
		Mode:                  runtimeconfig.ModeLocalEmulator,
		ProjectID:             projectID,
		FirestoreEmulatorHost: firestoreHost,
	})
	if err != nil {
		t.Fatalf("create cleanup runtime: %v", err)
	}
	eventID := ""
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if data, getErr := stateRuntime.Store.GetState(cleanupCtx, store.LostPetsCollection, petID); getErr == nil {
			var report domain.LostPetRecord
			if json.Unmarshal(data, &report) == nil && report.OwnerIdentityRef != "" {
				_ = stateRuntime.Store.DeleteState(cleanupCtx, store.ReportContactsCollection, report.OwnerIdentityRef)
			}
		}
		_ = stateRuntime.Store.DeleteState(cleanupCtx, store.LostPetsCollection, petID)
		if eventID != "" {
			_ = stateRuntime.Store.DeleteState(cleanupCtx, store.OutboxCollection, eventID)
		}
		_ = stateRuntime.Close()
	})

	lostPort := reserveLocalPort(t)
	lostProcess := startTestService(t, lostBinary, lostPort, projectID, firestoreHost, pubsubHost)
	t.Cleanup(func() { stopTestService(t, lostProcess) })
	waitForHTTP(t, ctx, "http://127.0.0.1:"+lostPort+"/lostPet")

	event := domain.LostPetEvent{
		PetID:         petID,
		ReporterEmail: "process-contract@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal lost pet event: %v", err)
	}
	response, err := http.Post(
		"http://127.0.0.1:"+lostPort+"/lostPet",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("post lost pet to service process: %v\nlogs:\n%s", err, lostProcess.logs.String())
	}
	responseBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("lost service status = %d, want %d; body = %s\nlogs:\n%s", response.StatusCode, http.StatusCreated, responseBody, lostProcess.logs.String())
	}
	var submission map[string]string
	if err := json.Unmarshal(responseBody, &submission); err != nil {
		t.Fatalf("decode lost service response: %v", err)
	}
	eventID = submission["eventId"]
	if eventID == "" {
		t.Fatal("lost service response did not contain eventId")
	}
	outboxRecord, err := outbox.GetRecord(ctx, stateRuntime.Store, eventID)
	if err != nil {
		t.Fatalf("read lost service outbox record: %v", err)
	}
	if outboxRecord.Status != outbox.StatusPublished {
		t.Fatalf("lost service outbox status = %q, want %q", outboxRecord.Status, outbox.StatusPublished)
	}

	webPort := reserveLocalPort(t)
	webProcess := startTestService(t, webBinary, webPort, projectID, firestoreHost, pubsubHost)
	t.Cleanup(func() { stopTestService(t, webProcess) })
	waitForHTTP(t, ctx, "http://127.0.0.1:"+webPort+"/healthz")
	assertWebProcessHasPet(t, webPort, petID, webProcess)

	stopTestService(t, webProcess)
	restartedWebProcess := startTestService(t, webBinary, webPort, projectID, firestoreHost, pubsubHost)
	t.Cleanup(func() { stopTestService(t, restartedWebProcess) })
	waitForHTTP(t, ctx, "http://127.0.0.1:"+webPort+"/healthz")
	assertWebProcessHasPet(t, webPort, petID, restartedWebProcess)
}

type testServiceProcess struct {
	command *exec.Cmd
	logs    lockedBuffer
	stopped bool
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func buildTestBinary(t *testing.T, ctx context.Context, root, output, packagePath string) {
	t.Helper()
	command := exec.CommandContext(ctx, "go", "build", "-o", output, packagePath)
	command.Dir = root
	combined, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", packagePath, err, combined)
	}
}

func startTestService(t *testing.T, binary, port, projectID, firestoreHost, pubsubHost string) *testServiceProcess {
	t.Helper()
	process := &testServiceProcess{}
	process.command = exec.Command(binary)
	process.command.Env = testServiceEnvironment(map[string]string{
		"PORT":                    port,
		"K_SERVICE":               "",
		"PETSPOTR_RUNTIME_MODE":   string(runtimeconfig.ModeLocalEmulator),
		"GOOGLE_CLOUD_PROJECT":    projectID,
		"FIRESTORE_EMULATOR_HOST": firestoreHost,
		"PUBSUB_EMULATOR_HOST":    pubsubHost,
	})
	process.command.Stdout = &process.logs
	process.command.Stderr = &process.logs
	if err := process.command.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}
	return process
}

func stopTestService(t *testing.T, process *testServiceProcess) {
	t.Helper()
	if process == nil || process.stopped || process.command.Process == nil {
		return
	}
	process.stopped = true
	if err := process.command.Process.Signal(syscall.SIGTERM); err != nil {
		if process.command.ProcessState == nil || !process.command.ProcessState.Exited() {
			t.Errorf("signal service process: %v\nlogs:\n%s", err, process.logs.String())
		}
		return
	}

	done := make(chan error, 1)
	go func() { done <- process.command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("service process exit: %v\nlogs:\n%s", err, process.logs.String())
		}
	case <-time.After(5 * time.Second):
		_ = process.command.Process.Kill()
		<-done
		t.Errorf("service process did not stop gracefully\nlogs:\n%s", process.logs.String())
	}
}

func reserveLocalPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved local port: %v", err)
	}
	return fmt.Sprintf("%d", port)
}

func waitForHTTP(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v", url, ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func assertWebProcessHasPet(t *testing.T, port, petID string, process *testServiceProcess) {
	t.Helper()
	response, err := http.Get("http://127.0.0.1:" + port + "/api/v1/lost-pets?limit=100")
	if err != nil {
		t.Fatalf("read lost pets from web process: %v\nlogs:\n%s", err, process.logs.String())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("web status = %d, want %d; body = %s\nlogs:\n%s", response.StatusCode, http.StatusOK, body, process.logs.String())
	}

	var pets []domain.LostPetEvent
	if err := json.NewDecoder(response.Body).Decode(&pets); err != nil {
		t.Fatalf("decode web lost pets: %v", err)
	}
	for _, pet := range pets {
		if pet.PetID == petID {
			return
		}
	}
	t.Fatalf("web process did not return %s; got %d pets\nlogs:\n%s", petID, len(pets), process.logs.String())
}

func testServiceEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, overridden := overrides[key]; overridden {
				continue
			}
		}
		environment = append(environment, item)
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}
