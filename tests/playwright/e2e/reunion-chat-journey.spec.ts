import { test, expect, BrowserContext, Page } from '@playwright/test';
import http from 'http';
import { AddressInfo } from 'net';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

const TEST_1X1_PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
  'base64'
);

interface SSEServer {
  port: number;
  broadcast: (event: string, data: unknown) => void;
  dropConnections: () => void;
  close: () => Promise<void>;
}

function startSSEServer(): Promise<SSEServer> {
  return new Promise((resolve, reject) => {
    const clients = new Set<http.ServerResponse>();
    const server = http.createServer((req, res) => {
      if (req.method === 'OPTIONS') {
        res.writeHead(204, {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
          'Access-Control-Allow-Headers': '*',
        });
        res.end();
        return;
      }

      const url = new URL(req.url || '', `http://${req.headers.host}`);
      if (url.pathname === '/api/v1/reunions/events') {
        res.writeHead(200, {
          'Content-Type': 'text/event-stream',
          'Cache-Control': 'no-cache, no-transform',
          'Connection': 'keep-alive',
          'Access-Control-Allow-Origin': '*',
        });
        res.write(': ping\n\n');
        clients.add(res);

        req.on('close', () => {
          clients.delete(res);
        });
        return;
      }

      res.writeHead(404);
      res.end();
    });

    server.listen(0, '127.0.0.1', () => {
      const addr = server.address() as AddressInfo;
      resolve({
        port: addr.port,
        broadcast: (event: string, data: unknown) => {
          const payload = `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
          for (const client of clients) {
            try {
              client.write(payload);
            } catch (_) {
              clients.delete(client);
            }
          }
        },
        dropConnections: () => {
          for (const client of clients) {
            try {
              client.destroy();
            } catch (_) {}
          }
          clients.clear();
        },
        close: () =>
          new Promise<void>((done) => {
            for (const client of clients) {
              try {
                client.end();
              } catch (_) {}
            }
            clients.clear();
            server.close(() => done());
          }),
      });
    });

    server.on('error', reject);
  });
}

function createMockMatch(matchId: string = 'match-e2e-realtime', status: string = 'CONFIRMED') {
  return {
    matchId,
    foundPetId: `found-${matchId}`,
    matchedPetId: `lost-${matchId}`,
    score: 0.95,
    status,
    matchedAt: '2026-09-18T12:00:00Z',
    scores: {
      vector: 0.98,
      trait: 0.94,
      visual: 0.94,
      color: 0.92,
      spatial: 0.96,
      distanceMiles: 0.5,
    },
    lostPet: {
      petId: `lost-${matchId}`,
      petName: 'Rusty',
      breed: 'Golden Retriever',
      location: 'Capitol Hill, Seattle, WA',
      imageUrl: 'https://storage.petspotr.io/images/lost-rusty.jpg',
      images: [
        { url: 'https://storage.petspotr.io/images/lost-rusty.jpg', tag: 'primary' },
      ],
    },
    foundPet: {
      petId: `found-${matchId}`,
      petName: 'Found Dog',
      breed: 'Golden Retriever',
      location: 'First Hill, Seattle, WA',
      imageUrl: 'https://storage.petspotr.io/images/found-dog.jpg',
      images: [
        { url: 'https://storage.petspotr.io/images/found-dog.jpg', tag: 'primary' },
      ],
    },
  };
}

test.describe('Multi-User Real-Time Reunion Chat & Resolution Journey', () => {
  let sseServer: SSEServer;

  test.beforeEach(async () => {
    sseServer = await startSSEServer();
  });

  test.afterEach(async () => {
    if (sseServer) {
      await sseServer.close();
    }
  });

  async function setupParticipantContext(
    context: BrowserContext,
    principal: { issuer: string; subject: string; email: string },
    getMatchStatus: () => string,
    messages: Array<{ messageId: string; senderRole: string; message: string; images?: string[]; sentAt: string }>,
    senderRole: 'reporter' | 'finder',
    onPresence?: (status: string) => void,
    onResolve?: () => void
  ): Promise<Page> {
    await context.addInitScript((port: number) => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: async () => 'mock-token',
        signOut: async () => {},
      });

      const OriginalEventSource = window.EventSource;
      class PatchedEventSource extends OriginalEventSource {
        constructor(url: string | URL, initDict?: EventSourceInit) {
          const urlStr = typeof url === 'string' ? url : url.toString();
          if (urlStr.includes('/api/v1/reunions/events')) {
            const parsed = new URL(urlStr, window.location.origin);
            super(`http://127.0.0.1:${port}${parsed.pathname}${parsed.search}`, initDict);
          } else {
            super(url, initDict);
          }
        }
      }
      window.EventSource = PatchedEventSource as unknown as typeof EventSource;
    }, sseServer.port);

    await context.route('**/api/v1/session/client-config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ enabled: true, provider: 'google.com' }),
      });
    });

    await context.route('**/api/v1/session/csrf', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ csrfToken: 'csrf-test-token' }),
      });
    });

    await context.route('**/api/v1/session', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(principal),
      });
    });

    await context.route('**/api/v1/matches', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([createMockMatch('match-e2e-realtime', getMatchStatus())]),
      });
    });

    await context.route('https://storage.petspotr.io/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'image/png',
        body: TEST_1X1_PNG,
      });
    });

    await context.route('**/mock-storage/**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'text/plain',
        body: 'OK',
      });
    });

    await context.route('**/api/v1/uploads/presigned-url', async (route) => {
      const data = route.request().postDataJSON();
      const fileName = data?.fileName || 'verification-photo.png';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          uploadUrl: `http://localhost:8082/mock-storage/${fileName}`,
          fileName: `images/reunions/${fileName}`,
        }),
      });
    });

    await context.route('**/api/v1/reunions/presence', async (route) => {
      const body = route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'ok' }),
      });
      if (onPresence) onPresence(body.status);
    });

    await context.route('**/api/v1/reunions/contact?*', async (route) => {
      const matchId = new URL(route.request().url()).searchParams.get('matchId') || 'match-e2e-realtime';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          matchId,
          messages,
        }),
      });
    });

    await context.route('**/api/v1/reunions/contact', async (route) => {
      const body = route.request().postDataJSON();
      const newMsg = {
        messageId: `msg-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        senderRole,
        message: body.message,
        images: body.images || [],
        sentAt: new Date().toISOString(),
      };
      messages.push(newMsg);
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'accepted',
          matchId: body.matchId,
          message: newMsg,
        }),
      });
      sseServer.broadcast('message', newMsg);
    });

    await context.route('**/api/v1/reunions/resolve', async (route) => {
      const body = route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'success',
          matchId: body.matchId,
        }),
      });
      if (onResolve) onResolve();
      sseServer.broadcast('reunion_resolved', { status: 'REUNITED' });
    });

    const page = await context.newPage();
    return page;
  }

  test('should stream private messages and typing indicators in real-time between owner and finder', async ({ browser }) => {
    const ownerContext = await browser.newContext({ bypassCSP: true });
    const finderContext = await browser.newContext({ bypassCSP: true });

    try {
      const sharedMessages: Array<{
        messageId: string;
        senderRole: string;
        message: string;
        images?: string[];
        sentAt: string;
      }> = [];

      let matchStatus = 'CONFIRMED';
      const getMatchStatus = () => matchStatus;

      const ownerPrincipal = {
        issuer: 'https://accounts.google.com',
        subject: 'owner-user',
        email: 'owner@example.com',
      };
      const finderPrincipal = {
        issuer: 'https://accounts.google.com',
        subject: 'finder-user',
        email: 'finder@example.com',
      };

      const ownerPage = await setupParticipantContext(
        ownerContext,
        ownerPrincipal,
        getMatchStatus,
        sharedMessages,
        'reporter'
      );

      const finderPage = await setupParticipantContext(
        finderContext,
        finderPrincipal,
        getMatchStatus,
        sharedMessages,
        'finder',
        (status) => {
          sseServer.broadcast('presence', {
            matchId: 'match-e2e-realtime',
            senderRole: 'finder',
            status,
          });
        }
      );

      // 1. Owner opens /matches and opens private messages
      await ownerPage.goto(`${WEB_FRONTEND_URL}/matches`);
      const ownerCard = ownerPage.locator('article[data-match-id="match-e2e-realtime"]');
      await expect(ownerCard).toBeVisible();
      await ownerCard.getByRole('button', { name: 'Open private messages' }).click();

      const ownerModal = ownerPage.locator('#match-thread-modal');
      await expect(ownerModal).toBeVisible();
      await expect(ownerPage.locator('#match-presence-text')).toHaveText('Online');
      await expect(ownerPage.locator('#match-presence-dot')).toHaveClass(/status-online/);

      // 2. Finder opens /matches and opens private messages
      await finderPage.goto(`${WEB_FRONTEND_URL}/matches`);
      const finderCard = finderPage.locator('article[data-match-id="match-e2e-realtime"]');
      await expect(finderCard).toBeVisible();
      await finderCard.getByRole('button', { name: 'Open private messages' }).click();

      const finderModal = finderPage.locator('#match-thread-modal');
      await expect(finderModal).toBeVisible();
      await expect(finderPage.locator('#match-presence-text')).toHaveText('Online');
      await expect(finderPage.locator('#match-presence-dot')).toHaveClass(/status-online/);

      // 3. Finder types in message input -> typing indicator sent -> Owner sees "Finder is typing..."
      await finderPage.locator('#match-thread-message').fill('Hello');
      await expect(ownerPage.locator('#match-thread-typing')).toBeVisible();
      await expect(ownerPage.locator('#match-thread-typing-text')).toHaveText('Finder is typing...');

      // 4. Finder sends message -> delivered via SSE to Owner in real-time without reload
      const sentMessageText = 'Hello, I found your dog near the park!';
      await finderPage.locator('#match-thread-message').fill(sentMessageText);
      await finderPage.locator('#match-thread-send').click();

      // Verify Owner receives message in real-time
      const ownerMessage = ownerPage.locator('#match-thread-messages .match-thread-message').first();
      await expect(ownerMessage).toBeVisible();
      await expect(ownerMessage).toContainText(sentMessageText);
      await expect(ownerMessage).toContainText('Finder');

      // Verify Finder also shows the message in thread
      const finderMessage = finderPage.locator('#match-thread-messages .match-thread-message').first();
      await expect(finderMessage).toBeVisible();
      await expect(finderMessage).toContainText(sentMessageText);
    } finally {
      await ownerContext.close();
      await finderContext.close();
    }
  });

  test('should share verification photos and mark reunion resolved inside chat', async ({ browser }) => {
    const ownerContext = await browser.newContext({ bypassCSP: true });
    const finderContext = await browser.newContext({ bypassCSP: true });

    try {
      const sharedMessages: Array<{
        messageId: string;
        senderRole: string;
        message: string;
        images?: string[];
        sentAt: string;
      }> = [];

      let matchStatus = 'CONFIRMED';
      const getMatchStatus = () => matchStatus;

      const ownerPrincipal = {
        issuer: 'https://accounts.google.com',
        subject: 'owner-user',
        email: 'owner@example.com',
      };
      const finderPrincipal = {
        issuer: 'https://accounts.google.com',
        subject: 'finder-user',
        email: 'finder@example.com',
      };

      const ownerPage = await setupParticipantContext(
        ownerContext,
        ownerPrincipal,
        getMatchStatus,
        sharedMessages,
        'reporter',
        undefined,
        () => {
          matchStatus = 'REUNITED';
        }
      );

      const finderPage = await setupParticipantContext(
        finderContext,
        finderPrincipal,
        getMatchStatus,
        sharedMessages,
        'finder',
        (status) => {
          sseServer.broadcast('presence', {
            matchId: 'match-e2e-realtime',
            senderRole: 'finder',
            status,
          });
        }
      );

      // Both users open /matches and enter Reunion Room
      await ownerPage.goto(`${WEB_FRONTEND_URL}/matches`);
      await ownerPage.locator('article[data-match-id="match-e2e-realtime"]')
        .getByRole('button', { name: 'Open private messages' }).click();
      await expect(ownerPage.locator('#match-thread-modal')).toBeVisible();
      await expect(ownerPage.locator('#match-presence-text')).toHaveText('Online');

      await finderPage.goto(`${WEB_FRONTEND_URL}/matches`);
      await finderPage.locator('article[data-match-id="match-e2e-realtime"]')
        .getByRole('button', { name: 'Open private messages' }).click();
      await expect(finderPage.locator('#match-thread-modal')).toBeVisible();
      await expect(finderPage.locator('#match-presence-text')).toHaveText('Online');

      // 1. Finder stages a verification photo attachment
      await finderPage.locator('#match-thread-file-input').setInputFiles({
        name: 'collar-verification.png',
        mimeType: 'image/png',
        buffer: TEST_1X1_PNG,
      });

      // Verify thumbnail displays in staged tray
      await expect(finderPage.locator('#match-thread-staged-tray')).toBeVisible();
      await expect(finderPage.locator('#match-thread-staged-thumbs .staged-thumb-chip')).toHaveCount(1);

      // 2. Finder sends message with attachment
      const photoMessageText = 'Here is a photo of the rabies tag and blue collar.';
      await finderPage.locator('#match-thread-message').fill(photoMessageText);
      await finderPage.locator('#match-thread-send').click();

      // Owner receives message with image thumbnail in real-time
      const ownerImgMessage = ownerPage.locator('#match-thread-messages .match-thread-message').last();
      await expect(ownerImgMessage).toBeVisible();
      await expect(ownerImgMessage).toContainText(photoMessageText);
      const thumbnail = ownerImgMessage.locator('img.thumbnail-img');
      await expect(thumbnail).toBeVisible();
      await expect(thumbnail).toHaveAttribute('src', /collar-verification\.png/);

      // 3. Owner clicks "Mark as Reunited" inside chat modal
      await ownerPage.locator('#btn-chat-resolve-reunion').click();

      // Submit the reunion confirmation modal
      await expect(ownerPage.locator('#reunion-modal')).toBeVisible();
      await ownerPage.locator('#btn-submit-reunion').click();

      // Dismiss the action confirmation modal (closing it if visible)
      await ownerPage.evaluate(() => {
        const modal = document.getElementById('match-action-modal');
        if (modal) modal.hidden = true;
      });

      // 4. Verify both Owner and Finder receive reunion_resolved event:
      // Celebratory banner appears in both windows
      await expect(ownerPage.locator('#match-thread-resolved-banner')).toBeVisible();
      await expect(finderPage.locator('#match-thread-resolved-banner')).toBeVisible();

      // Compose form is hidden and conversation marked read-only in both windows
      await expect(ownerPage.locator('#match-thread-form')).toBeHidden();
      await expect(ownerPage.locator('#match-thread-readonly')).toBeVisible();

      await expect(finderPage.locator('#match-thread-form')).toBeHidden();
      await expect(finderPage.locator('#match-thread-readonly')).toBeVisible();
    } finally {
      await ownerContext.close();
      await finderContext.close();
    }
  });

  test('should merge real-time and catch-up messages safely and filter untrusted image hosts', async ({ browser }) => {
    const ownerContext = await browser.newContext({ bypassCSP: true });

    try {
      const messages: Array<{
        messageId: string;
        senderRole: string;
        message: string;
        images?: string[];
        sentAt: string;
      }> = [
        {
          messageId: 'msg-initial-1',
          senderRole: 'reporter',
          message: 'Initial message from owner',
          sentAt: new Date(Date.now() - 60000).toISOString(),
        },
      ];

      const matchStatus = 'CONFIRMED';
      const getMatchStatus = () => matchStatus;

      const ownerPage = await setupParticipantContext(
        ownerContext,
        {
          issuer: 'https://accounts.google.com',
          subject: 'owner-sub-1',
          email: 'owner@example.com',
        },
        getMatchStatus,
        messages,
        'reporter'
      );

      await ownerPage.goto(`${WEB_FRONTEND_URL}/matches`);
      await ownerPage.locator('article[data-match-id="match-e2e-realtime"]')
        .getByRole('button', { name: 'Open private messages' }).click();
      await expect(ownerPage.locator('#match-thread-modal')).toBeVisible();
      await expect(ownerPage.locator('#match-presence-text')).toHaveText('Online');

      // Verify initial message is displayed
      const initialEl = ownerPage.locator('#match-thread-messages .match-thread-message[data-message-id="msg-initial-1"]');
      await expect(initialEl).toBeVisible();

      // Broadcast an SSE message (real-time arrival while thread was loaded)
      const sseMsg = {
        messageId: 'msg-sse-realtime',
        senderRole: 'finder',
        message: 'Real-time message while connected',
        sentAt: new Date(Date.now() - 30000).toISOString(),
      };
      sseServer.broadcast('message', sseMsg);

      // Verify the SSE message appears in the DOM
      const sseEl = ownerPage.locator('#match-thread-messages .match-thread-message[data-message-id="msg-sse-realtime"]');
      await expect(sseEl).toBeVisible();

      // Add a catch-up message to backend history that includes both trusted and untrusted image URLs
      const catchUpMsg = {
        messageId: 'msg-catchup-3',
        senderRole: 'finder',
        message: 'Catch-up message with image attachments',
        images: [
          'images/reunions/trusted-collar.png',
          'http://evil.com/exploit.png',
          'https://untrusted-host.example.com/tracker.png',
        ],
        sentAt: new Date().toISOString(),
      };
      messages.push(catchUpMsg);

      // Drop SSE connection so EventSource reconnects and fires 'open', triggering history catch-up
      sseServer.dropConnections();

      // Wait for EventSource to reconnect and status to return to 'Online'
      await expect(ownerPage.locator('#match-presence-text')).toHaveText('Online');

      // Verify catch-up message appeared
      const catchUpEl = ownerPage.locator('#match-thread-messages .match-thread-message[data-message-id="msg-catchup-3"]');
      await expect(catchUpEl).toBeVisible();

      // Verify the real-time SSE message was NOT dropped or destroyed during catch-up!
      await expect(sseEl).toBeVisible();
      await expect(initialEl).toBeVisible();

      // Verify all 3 messages are displayed in DOM
      await expect(ownerPage.locator('#match-thread-messages .match-thread-message')).toHaveCount(3);

      // Verify image filtering: only the trusted image from allowed host/relative path is rendered
      // http://evil.com and https://untrusted-host.example.com must be rejected
      const thumbnails = catchUpEl.locator('img.thumbnail-img');
      await expect(thumbnails).toHaveCount(1);
      await expect(thumbnails.first()).toHaveAttribute('src', /storage\.petspotr\.io\/images\/reunions\/trusted-collar\.png/);
    } finally {
      await ownerContext.close();
    }
  });
});
