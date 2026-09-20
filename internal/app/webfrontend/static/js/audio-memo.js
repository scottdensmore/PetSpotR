/**
 * PetSpotR - Multilingual & Audio Voice Memo Module (Milestone 8.4)
 * Handles client-side audio recording (MediaRecorder up to 15s),
 * waveform visualization, ARIA live announcements, and language selection.
 */

(function () {
  'use strict';

  // --- ARIA Announcer Helper ---
  function announce(message) {
    const announcer = document.getElementById('aria-announcer');
    if (!announcer) return;
    announcer.textContent = '';
    setTimeout(() => {
      announcer.textContent = message;
    }, 50);
  }

  // Expose global announcer
  window.petspotrAnnounce = announce;

  // --- Language Selector Handling ---
  function initLanguageSelector() {
    const langSelect = document.getElementById('lang-select');
    if (!langSelect) return;

    langSelect.addEventListener('change', (e) => {
      const selectedLocale = e.target.value;
      if (!selectedLocale) return;

      // Set cookie for 1 year
      document.cookie = `petspotr_locale=${selectedLocale}; path=/; max-age=31536000; SameSite=Lax`;

      announce(`Language changed to ${selectedLocale}`);

      // Reload current page with ?lang= parameter
      const url = new URL(window.location.href);
      url.searchParams.set('lang', selectedLocale);
      window.location.href = url.toString();
    });
  }

  // --- Audio Voice Memo Recorder ---
  class VoiceMemoRecorder {
    constructor() {
      this.mediaRecorder = null;
      this.audioChunks = [];
      this.audioBlob = null;
      this.duration = 0;
      this.timerInterval = null;
      this.startTime = null;
      this.maxDurationSeconds = 15;
      this.waveform = [];

      this.btnRecord = document.getElementById('btn-record-voice-memo');
      this.btnStop = document.getElementById('btn-stop-voice-memo');
      this.btnDiscard = document.getElementById('btn-discard-voice-memo');
      this.timerDisplay = document.getElementById('voice-memo-timer');
      this.progressContainer = document.getElementById('voice-memo-progress-bar-container');
      this.progressBar = document.getElementById('voice-memo-progress-bar');
      this.previewContainer = document.getElementById('voice-memo-preview-container');
      this.audioPlayer = document.getElementById('voice-memo-audio');
      this.waveformVisualizer = document.getElementById('voice-memo-waveform-bars');
      this.inputUrl = document.getElementById('sighting-voice-memo-url');
      this.inputDuration = document.getElementById('sighting-voice-memo-duration');

      if (this.btnRecord) {
        this.bindEvents();
      }
    }

    bindEvents() {
      this.btnRecord.addEventListener('click', () => this.startRecording());
      if (this.btnStop) {
        this.btnStop.addEventListener('click', () => this.stopRecording());
      }
      if (this.btnDiscard) {
        this.btnDiscard.addEventListener('click', () => this.discardRecording());
      }
    }

    async startRecording() {
      try {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
          alert('Audio recording is not supported on this device/browser.');
          return;
        }

        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        this.audioChunks = [];
        this.audioBlob = null;
        this.duration = 0;

        const mimeTypes = ['audio/webm', 'audio/ogg', 'audio/mp4', 'audio/wav'];
        let selectedMime = '';
        for (const mime of mimeTypes) {
          if (MediaRecorder.isTypeSupported && MediaRecorder.isTypeSupported(mime)) {
            selectedMime = mime;
            break;
          }
        }

        const options = selectedMime ? { mimeType: selectedMime } : {};
        this.mediaRecorder = new MediaRecorder(stream, options);

        this.mediaRecorder.ondataavailable = (event) => {
          if (event.data && event.data.size > 0) {
            this.audioChunks.push(event.data);
          }
        };

        this.mediaRecorder.onstop = () => {
          this.handleRecordingStopped(stream);
        };

        this.mediaRecorder.start(100);
        this.startTime = Date.now();

        // Update UI
        this.btnRecord.classList.add('hidden');
        if (this.btnStop) this.btnStop.classList.remove('hidden');
        if (this.btnDiscard) this.btnDiscard.classList.add('hidden');
        if (this.progressContainer) this.progressContainer.classList.remove('hidden');
        if (this.previewContainer) this.previewContainer.classList.add('hidden');

        announce('Voice memo recording started. Maximum 15 seconds.');

        this.startTimer();
      } catch (err) {
        console.error('Failed to start voice recording:', err);
        announce('Failed to access microphone.');
      }
    }

    startTimer() {
      if (this.timerInterval) clearInterval(this.timerInterval);

      this.timerInterval = setInterval(() => {
        const elapsedMs = Date.now() - this.startTime;
        const elapsedSec = Math.min(this.maxDurationSeconds, elapsedMs / 1000);
        this.duration = elapsedSec;

        const formatted = this.formatTime(elapsedSec);
        if (this.timerDisplay) {
          this.timerDisplay.textContent = `${formatted} / 0:15`;
        }

        const percentage = Math.min(100, (elapsedSec / this.maxDurationSeconds) * 100);
        if (this.progressBar) {
          this.progressBar.style.width = `${percentage}%`;
        }
        if (this.progressContainer) {
          this.progressContainer.setAttribute('aria-valuenow', elapsedSec.toFixed(1));
        }

        if (elapsedSec >= this.maxDurationSeconds) {
          this.stopRecording();
        }
      }, 100);
    }

    formatTime(seconds) {
      const s = Math.floor(seconds);
      const dec = Math.floor((seconds % 1) * 10);
      return `0:${s < 10 ? '0' : ''}${s}.${dec}`;
    }

    stopRecording() {
      if (this.timerInterval) {
        clearInterval(this.timerInterval);
        this.timerInterval = null;
      }

      if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
        this.mediaRecorder.stop();
      }

      if (this.btnStop) this.btnStop.classList.add('hidden');
      if (this.btnRecord) this.btnRecord.classList.remove('hidden');
      if (this.btnDiscard) this.btnDiscard.classList.remove('hidden');

      announce('Voice memo recording stopped.');
    }

    async handleRecordingStopped(stream) {
      // Stop all audio tracks to release microphone
      if (stream) {
        stream.getTracks().forEach((track) => track.stop());
      }

      const mimeType = (this.mediaRecorder && this.mediaRecorder.mimeType) || 'audio/webm';
      this.audioBlob = new Blob(this.audioChunks, { type: mimeType });

      if (this.audioPlayer) {
        const audioUrl = URL.createObjectURL(this.audioBlob);
        this.audioPlayer.src = audioUrl;
      }

      if (this.previewContainer) {
        this.previewContainer.classList.remove('hidden');
      }

      // Generate waveform
      await this.generateWaveform();

      announce(`Voice memo recorded successfully (${this.duration.toFixed(1)} seconds).`);
    }

    async generateWaveform() {
      this.waveform = [];
      const sampleCount = 30;

      try {
        if (window.AudioContext || window.webkitAudioContext) {
          const AudioCtx = window.AudioContext || window.webkitAudioContext;
          const ctx = new AudioCtx();
          const arrayBuffer = await this.audioBlob.arrayBuffer();
          const audioBuffer = await ctx.decodeAudioData(arrayBuffer);
          const rawData = audioBuffer.getChannelData(0);
          const blockSize = Math.floor(rawData.length / sampleCount);

          let maxVal = 0;
          for (let i = 0; i < sampleCount; i++) {
            let blockStart = i * blockSize;
            let sum = 0;
            for (let j = 0; j < blockSize; j++) {
              sum += Math.abs(rawData[blockStart + j] || 0);
            }
            const avg = sum / blockSize;
            this.waveform.push(avg);
            if (avg > maxVal) maxVal = avg;
          }

          if (maxVal > 0) {
            this.waveform = this.waveform.map((v) => Math.max(0.08, Math.round((v / maxVal) * 100) / 100));
          }
          await ctx.close();
        }
      } catch (e) {
        // Fallback simulated waveform
        for (let i = 0; i < sampleCount; i++) {
          this.waveform.push(Math.round((0.15 + Math.random() * 0.75) * 100) / 100);
        }
      }

      if (this.waveform.length === 0) {
        for (let i = 0; i < sampleCount; i++) {
          this.waveform.push(0.2);
        }
      }

      this.renderWaveform();
    }

    renderWaveform() {
      if (!this.waveformVisualizer) return;
      this.waveformVisualizer.innerHTML = '';

      this.waveform.forEach((val, idx) => {
        const bar = document.createElement('div');
        bar.className = 'waveform-bar';
        bar.style.height = `${Math.max(10, Math.min(100, val * 100))}%`;
        bar.setAttribute('data-index', idx);
        this.waveformVisualizer.appendChild(bar);
      });
    }

    discardRecording() {
      this.audioChunks = [];
      this.audioBlob = null;
      this.duration = 0;
      this.waveform = [];

      if (this.timerInterval) {
        clearInterval(this.timerInterval);
        this.timerInterval = null;
      }

      if (this.audioPlayer) {
        this.audioPlayer.pause();
        this.audioPlayer.src = '';
      }

      if (this.timerDisplay) {
        this.timerDisplay.textContent = '0:00 / 0:15';
      }
      if (this.progressBar) {
        this.progressBar.style.width = '0%';
      }
      if (this.progressContainer) {
        this.progressContainer.classList.add('hidden');
        this.progressContainer.setAttribute('aria-valuenow', '0');
      }
      if (this.previewContainer) {
        this.previewContainer.classList.add('hidden');
      }
      if (this.btnDiscard) {
        this.btnDiscard.classList.add('hidden');
      }
      if (this.inputUrl) {
        this.inputUrl.value = '';
      }
      if (this.inputDuration) {
        this.inputDuration.value = '';
      }

      announce('Voice memo discarded.');
    }

    async uploadVoiceMemo(petId, sightingId) {
      if (!this.audioBlob) return null;

      const formData = new FormData();
      const ext = this.audioBlob.type.includes('wav') ? 'wav' : 'webm';
      formData.append('audio', this.audioBlob, `sighting-note.${ext}`);
      formData.append('duration', this.duration.toFixed(2));

      const res = await fetch(`/api/v1/lost-pets/${encodeURIComponent(petId)}/sightings/${encodeURIComponent(sightingId)}/voice-memo`, {
        method: 'POST',
        body: formData,
      });

      if (!res.ok) {
        throw new Error(`Upload failed with status ${res.status}`);
      }

      const data = await res.json();
      if (this.inputUrl) this.inputUrl.value = data.voiceMemoUrl;
      if (this.inputDuration) this.inputDuration.value = data.durationSeconds;
      announce('Voice memo uploaded successfully.');
      return data;
    }
  }

  // --- Document Ready Initialization ---
  document.addEventListener('DOMContentLoaded', () => {
    initLanguageSelector();
    window.petspotrVoiceRecorder = new VoiceMemoRecorder();
  });
})();
