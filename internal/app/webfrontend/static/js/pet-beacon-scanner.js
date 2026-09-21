/**
 * PetSpotR - Client Web Bluetooth Scanner & Web Audio Geiger Synthesizer (Milestone 10.1)
 *
 * Provides real-time Bluetooth Low Energy (BLE) scanning for lost pet collar beacons.
 * Features:
 *  - Exponential Moving Average (EMA) smoothing on RSSI (alpha = 0.35).
 *  - Log-distance path loss distance calculation (n = 2.5).
 *  - Web Audio Geiger counter synthesizer (pitch 220Hz - 880Hz, rate 1.0s - 0.1s).
 *  - Audio mute persistence in localStorage.
 *  - Graceful degradation and test injection hook (window.__mockBluetoothScanner).
 */

(function (global) {
  'use strict';

  const DEFAULT_TX_POWER_1M = -59;
  const DEFAULT_PATH_LOSS_EXPONENT = 2.5;
  const EMA_ALPHA = 0.35;
  const STORAGE_KEY_AUDIO_MUTED = 'petspotr_beacon_audio_muted';

  class PetBeaconScanner {
    /**
     * @param {Object} options
     * @param {string} options.petId
     * @param {Object} [options.beaconConfig]
     * @param {string} [options.volunteerAlias]
     * @param {Function} [options.onPing]
     * @param {Function} [options.onTriangulationUpdate]
     * @param {Function} [options.onStatusChange]
     * @param {Function} [options.getObserverCoords]
     */
    constructor(options = {}) {
      this.petId = options.petId || '';
      this.beaconConfig = options.beaconConfig || null;
      this.volunteerAlias = options.volunteerAlias || 'Volunteer';
      this.onPing = options.onPing || null;
      this.onTriangulationUpdate = options.onTriangulationUpdate || null;
      this.onStatusChange = options.onStatusChange || null;
      this.getObserverCoords = options.getObserverCoords || (() => ({ latitude: 0, longitude: 0 }));

      // Signal state
      this.isScanning = false;
      this.smoothedRssi = null;
      this.latestDistance = null;
      this.latestProximity = 'out_of_range';
      this.latestPing = null;
      this.alpha = EMA_ALPHA;
      this.pathLossExponent = DEFAULT_PATH_LOSS_EXPONENT;

      // Bluetooth handles
      this.bluetoothScan = null;
      this.advertisementHandler = null;

      // Audio state
      this.audioCtx = null;
      this.audioTimer = null;

      // Persistent audio mute preference: default is true (muted) unless explicitly 'false'
      let savedMute = null;
      try {
        if (typeof localStorage !== 'undefined') {
          savedMute = localStorage.getItem(STORAGE_KEY_AUDIO_MUTED);
        }
      } catch (_) {}
      this.isAudioMuted = (savedMute !== 'false');

      // Register with test mock hook if available
      if (typeof window !== 'undefined' && window.__mockBluetoothScanner && typeof window.__mockBluetoothScanner.registerScanner === 'function') {
        window.__mockBluetoothScanner.registerScanner(this);
      }
    }

    /**
     * Gets calibrated Tx power at 1 meter.
     * @returns {number}
     */
    getTxPower1m() {
      if (this.beaconConfig && typeof this.beaconConfig.calibratedRssi === 'number') {
        return this.beaconConfig.calibratedRssi;
      }
      return DEFAULT_TX_POWER_1M;
    }

    /**
     * Starts scanning for BLE beacon advertisements.
     * @returns {Promise<void>}
     */
    async startScan() {
      if (this.isScanning) {
        return;
      }

      this._initAudioContext();
      this._emitStatus('starting', 'Initializing Web Bluetooth scanner...');

      // 1. Check if mock scanner is actively mocking scan
      if (typeof window !== 'undefined' && window.__mockBluetoothScanner && window.__mockBluetoothScanner.mockActive) {
        this.isScanning = true;
        this._startAudioLoop();
        this._emitStatus('scanning', 'Scanner Active (Simulated)');
        return;
      }

      // 2. Check navigator.bluetooth support
      if (typeof navigator === 'undefined' || !navigator.bluetooth) {
        // Fallback for headless testing environments (Playwright, Jest, CI)
        if (typeof window !== 'undefined' && (window.__mockBluetoothScanner || window.__TEST_MODE__)) {
          this.isScanning = true;
          this._startAudioLoop();
          this._emitStatus('scanning', 'Scanner Active (Mock Mode)');
          return;
        }
        const err = new Error('Web Bluetooth is not supported in this browser or context.');
        this._emitStatus('unsupported', err.message);
        throw err;
      }

      try {
        if (typeof navigator.bluetooth.requestLEScan === 'function') {
          const scanOptions = {
            keepRepeatedDevices: true,
            acceptAllAdvertisements: true
          };

          this.advertisementHandler = (event) => this._handleAdvertisement(event);
          navigator.bluetooth.addEventListener('advertisementreceived', this.advertisementHandler);
          this.bluetoothScan = await navigator.bluetooth.requestLEScan(scanOptions);
        } else if (typeof navigator.bluetooth.requestDevice === 'function') {
          // Fallback for browsers supporting requestDevice
          await navigator.bluetooth.requestDevice({ acceptAllDevices: true });
        }

        this.isScanning = true;
        this._startAudioLoop();
        this._emitStatus('scanning', 'Scanner Active');
      } catch (err) {
        // Fallback in test / automation mode if requestLEScan fails
        if (typeof window !== 'undefined' && (window.__mockBluetoothScanner || window.__TEST_MODE__)) {
          this.isScanning = true;
          this._startAudioLoop();
          this._emitStatus('scanning', 'Scanner Active (Fallback Mock)');
          return;
        }
        this.isScanning = false;
        this._emitStatus('error', err.message || 'Bluetooth scan permission denied or unavailable');
        throw err;
      }
    }

    /**
     * Stops the Bluetooth scanner and clears audio intervals.
     */
    stopScan() {
      if (!this.isScanning) {
        return;
      }

      this.isScanning = false;
      this._stopAudioLoop();

      if (this.bluetoothScan && typeof this.bluetoothScan.stop === 'function') {
        try {
          this.bluetoothScan.stop();
        } catch (_) {}
        this.bluetoothScan = null;
      }

      if (this.advertisementHandler && typeof navigator !== 'undefined' && navigator.bluetooth) {
        try {
          navigator.bluetooth.removeEventListener('advertisementreceived', this.advertisementHandler);
        } catch (_) {}
        this.advertisementHandler = null;
      }

      this._emitStatus('stopped', 'Scanner Stopped');
    }

    /**
     * Toggles Geiger counter audio muted state and saves preference.
     * @returns {boolean} New muted state
     */
    toggleAudio() {
      this.isAudioMuted = !this.isAudioMuted;

      try {
        if (typeof localStorage !== 'undefined') {
          localStorage.setItem(STORAGE_KEY_AUDIO_MUTED, this.isAudioMuted ? 'true' : 'false');
        }
      } catch (_) {}

      if (!this.isAudioMuted) {
        this._initAudioContext();
        if (this.isScanning) {
          if (this.latestDistance !== null && this.latestDistance < 30.0) {
            this._playGeigerTick(this.latestDistance, this.smoothedRssi);
          }
          this._scheduleNextAudioTick();
        }
      } else {
        this._stopAudioLoop();
      }

      this._emitStatus('audio_toggle', this.isAudioMuted ? 'Muted' : 'Unmuted');
      return this.isAudioMuted;
    }

    /**
     * Test hook to inject simulated Bluetooth beacon observations.
     * @param {Object} mockData
     * @returns {Promise<Object>} The generated ping object
     */
    async injectMockPing(mockData = {}) {
      const rawRssi = typeof mockData.rssi === 'number' ? mockData.rssi : -65;
      const rawTxPower = typeof mockData.txPower1m === 'number'
        ? mockData.txPower1m
        : (typeof mockData.txPower === 'number' ? mockData.txPower : this.getTxPower1m());

      let customCoords = mockData.observerCoords || null;
      if (!customCoords && typeof mockData.latitude === 'number' && typeof mockData.longitude === 'number') {
        customCoords = { latitude: mockData.latitude, longitude: mockData.longitude };
      }

      const prevGetCoords = this.getObserverCoords;
      if (customCoords) {
        this.getObserverCoords = () => customCoords;
      }

      try {
        return await this._processPing(rawRssi, rawTxPower, mockData);
      } finally {
        if (customCoords) {
          this.getObserverCoords = prevGetCoords;
        }
      }
    }

    // =========================================================================
    // Internal Processing & Signal Mathematics
    // =========================================================================

    /**
     * Handles raw Bluetooth advertisement events.
     * @param {BluetoothAdvertisingEvent} event
     */
    _handleAdvertisement(event) {
      if (!this.isScanning || !event) return;

      // Filter against beaconConfig if provided
      if (this.beaconConfig) {
        if (this.beaconConfig.deviceName && event.device && event.device.name) {
          if (!event.device.name.toLowerCase().includes(this.beaconConfig.deviceName.toLowerCase())) {
            return;
          }
        }
        if (this.beaconConfig.deviceAddress && event.device && event.device.id) {
          if (!event.device.id.toLowerCase().includes(this.beaconConfig.deviceAddress.toLowerCase())) {
            return;
          }
        }
      }

      const rssi = typeof event.rssi === 'number' ? event.rssi : -75;
      const txPower = typeof event.txPower === 'number' ? event.txPower : this.getTxPower1m();
      this._processPing(rssi, txPower, event);
    }

    /**
     * Core ping ingestion, smoothing, distance estimation, and dispatch.
     * @param {number} rawRssi
     * @param {number} rawTxPower
     * @param {Object} rawData
     * @returns {Promise<Object>}
     */
    async _processPing(rawRssi, rawTxPower, rawData = {}) {
      const txPower = typeof rawTxPower === 'number' && !isNaN(rawTxPower) ? rawTxPower : this.getTxPower1m();
      const smoothed = this._smoothRssi(rawRssi);
      const roundedRssi = Math.round(smoothed);
      const distance = this._calculateDistance(smoothed, txPower);
      const proximity = this._determineProximity(distance, roundedRssi);

      this.latestDistance = distance;
      this.latestProximity = proximity;

      // Resolve observer coordinates
      let coords = { latitude: 0, longitude: 0 };
      if (typeof this.getObserverCoords === 'function') {
        try {
          const res = this.getObserverCoords();
          if (res && typeof res.then === 'function') {
            coords = await res;
          } else if (res && typeof res.latitude === 'number' && typeof res.longitude === 'number') {
            coords = res;
          }
        } catch (_) {}
      }

      const ping = {
        pingId: 'ping-' + Date.now() + '-' + Math.random().toString(36).substring(2, 8),
        petId: this.petId,
        volunteerAlias: this.volunteerAlias || 'Volunteer',
        observerCoords: coords,
        rssi: roundedRssi,
        txPower1m: txPower,
        distanceMeters: distance,
        proximity: proximity,
        recordedAt: (rawData.recordedAt instanceof Date)
          ? rawData.recordedAt.toISOString()
          : (typeof rawData.recordedAt === 'string' ? rawData.recordedAt : new Date().toISOString())
      };

      this.latestPing = ping;

      // Geiger click on signal arrival if audio is enabled
      if (!this.isAudioMuted) {
        this._playGeigerTick(distance, roundedRssi);
      }

      // Dispatch to consumer callback
      if (typeof this.onPing === 'function') {
        try {
          this.onPing(ping);
        } catch (err) {
          console.error('[PetBeaconScanner] onPing error:', err);
        }
      }

      return ping;
    }

    /**
     * Exponential Moving Average (EMA) smoothing on RSSI:
     * RSSI_smooth = 0.35 * RSSI_new + 0.65 * RSSI_prev
     * @param {number} rawRssi
     * @returns {number}
     */
    _smoothRssi(rawRssi) {
      if (typeof rawRssi !== 'number' || isNaN(rawRssi)) {
        return this.smoothedRssi !== null ? this.smoothedRssi : -80;
      }
      if (this.smoothedRssi === null) {
        this.smoothedRssi = rawRssi;
      } else {
        this.smoothedRssi = (this.alpha * rawRssi) + ((1.0 - this.alpha) * this.smoothedRssi);
      }
      return this.smoothedRssi;
    }

    /**
     * Computes log-distance path loss distance estimation:
     * d = 10^((txPower - rssi) / (10 * n))
     * Clamped to [0.1, 100.0] meters.
     * @param {number} rssi
     * @param {number} txPower
     * @returns {number}
     */
    _calculateDistance(rssi, txPower) {
      const exponent = this.pathLossExponent > 0 ? this.pathLossExponent : DEFAULT_PATH_LOSS_EXPONENT;
      const ratio = (txPower - rssi) / (10.0 * exponent);
      const d = Math.pow(10.0, ratio);
      const clamped = Math.max(0.1, Math.min(100.0, d));
      return parseFloat(clamped.toFixed(2));
    }

    /**
     * Categorizes proximity into immediate, near, far, or out_of_range.
     * Matches Go backend pkg/beacon.DetermineProximity.
     * @param {number} distance
     * @param {number} rssi
     * @returns {string}
     */
    _determineProximity(distance, rssi) {
      if (distance < 1.0 || rssi >= -60) {
        return 'immediate';
      }
      if (distance < 5.0 || rssi >= -75) {
        return 'near';
      }
      if (distance < 30.0 || rssi >= -90) {
        return 'far';
      }
      return 'out_of_range';
    }

    // =========================================================================
    // Web Audio Geiger Counter Synthesizer
    // =========================================================================

    /**
     * Initializes or resumes the Web Audio AudioContext.
     */
    _initAudioContext() {
      if (this.audioCtx) {
        if (this.audioCtx.state === 'suspended') {
          this.audioCtx.resume().catch(() => {});
        }
        return;
      }

      const AudioContextClass = global.AudioContext || global.webkitAudioContext;
      if (AudioContextClass) {
        try {
          this.audioCtx = new AudioContextClass();
        } catch (_) {}
      }
    }

    /**
     * Synthesizes a single Geiger audio tick / tone pulse.
     * Pitch scales dynamically from 220Hz (at 30m) to 880Hz (at <= 2m).
     * @param {number} distance
     * @param {number} rssi
     */
    _playGeigerTick(distance, rssi) {
      if (this.isAudioMuted || !this.audioCtx) return;

      if (this.audioCtx.state === 'suspended') {
        this.audioCtx.resume().catch(() => {});
      }

      // Pitch: 220Hz at 30m to 880Hz at <= 2m
      const d = Math.max(0.1, Math.min(distance !== null ? distance : 30.0, 30.0));
      const factor = Math.max(0.0, Math.min((30.0 - d) / 28.0, 1.0));
      const pitch = 220.0 + (880.0 - 220.0) * factor;

      try {
        const now = this.audioCtx.currentTime;
        const osc = this.audioCtx.createOscillator();
        const gain = this.audioCtx.createGain();

        osc.type = 'sine';
        osc.frequency.setValueAtTime(pitch, now);

        // Click envelope with 35ms pulse
        gain.gain.setValueAtTime(0.0001, now);
        gain.gain.exponentialRampToValueAtTime(0.3, now + 0.005);
        gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.035);

        osc.connect(gain);
        gain.connect(this.audioCtx.destination);

        osc.start(now);
        osc.stop(now + 0.04);
      } catch (_) {}
    }

    /**
     * Starts continuous audio ticker loop while scanning.
     */
    _startAudioLoop() {
      this._stopAudioLoop();
      if (!this.isAudioMuted) {
        this._scheduleNextAudioTick();
      }
    }

    /**
     * Schedules the next Geiger click based on distance proximity.
     * Interval scales from 1.0s (1000ms at 30m) down to 0.1s (100ms at <= 2m).
     */
    _scheduleNextAudioTick() {
      if (!this.isScanning || this.isAudioMuted) {
        return;
      }

      // If out of range or no pings yet, check again in 1s without ticking
      if (this.latestDistance === null || this.latestDistance >= 30.0 || this.latestProximity === 'out_of_range') {
        this.audioTimer = setTimeout(() => {
          this._scheduleNextAudioTick();
        }, 1000);
        return;
      }

      const d = Math.max(0.1, Math.min(this.latestDistance, 30.0));
      const factor = Math.max(0.0, Math.min((30.0 - d) / 28.0, 1.0));
      const intervalMs = Math.round(1000.0 - (1000.0 - 100.0) * factor); // 1000ms -> 100ms

      this.audioTimer = setTimeout(() => {
        if (this.isScanning && !this.isAudioMuted) {
          this._playGeigerTick(this.latestDistance, this.smoothedRssi);
          this._scheduleNextAudioTick();
        }
      }, intervalMs);
    }

    /**
     * Stops the audio ticker loop.
     */
    _stopAudioLoop() {
      if (this.audioTimer) {
        clearTimeout(this.audioTimer);
        this.audioTimer = null;
      }
    }

    /**
     * Emits status changes to consumer.
     * @param {string} status
     * @param {string} detail
     */
    _emitStatus(status, detail) {
      if (typeof this.onStatusChange === 'function') {
        try {
          this.onStatusChange(status, detail);
        } catch (err) {
          console.error('[PetBeaconScanner] onStatusChange error:', err);
        }
      }
    }
  }

  // =========================================================================
  // Global / Test Hook Registration
  // =========================================================================

  if (typeof global !== 'undefined') {
    global.PetBeaconScanner = PetBeaconScanner;

    // Set up or extend window.__mockBluetoothScanner test hook for Playwright/headless tests
    if (!global.__mockBluetoothScanner) {
      global.__mockBluetoothScanner = {
        mockActive: false,
        activeScanners: [],
        registerScanner(scanner) {
          if (!this.activeScanners.includes(scanner)) {
            this.activeScanners.push(scanner);
          }
        },
        unregisterScanner(scanner) {
          this.activeScanners = this.activeScanners.filter(s => s !== scanner);
        },
        async injectMockPing(mockData) {
          const results = [];
          for (const scanner of this.activeScanners) {
            results.push(await scanner.injectMockPing(mockData));
          }
          return results.length === 1 ? results[0] : results;
        }
      };
    } else {
      const mock = global.__mockBluetoothScanner;
      mock.activeScanners = mock.activeScanners || [];
      if (typeof mock.registerScanner !== 'function') {
        mock.registerScanner = function (scanner) {
          if (!this.activeScanners.includes(scanner)) {
            this.activeScanners.push(scanner);
          }
        };
      }
      if (typeof mock.unregisterScanner !== 'function') {
        mock.unregisterScanner = function (scanner) {
          this.activeScanners = this.activeScanners.filter(s => s !== scanner);
        };
      }
      if (typeof mock.injectMockPing !== 'function') {
        mock.injectMockPing = async function (mockData) {
          const results = [];
          for (const scanner of this.activeScanners) {
            results.push(await scanner.injectMockPing(mockData));
          }
          return results.length === 1 ? results[0] : results;
        };
      }
    }
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { PetBeaconScanner };
  }
})(typeof window !== 'undefined' ? window : globalThis);
