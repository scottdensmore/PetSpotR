/**
 * PetSpotR - Client WebRTC DataChannel Controller & Offline IndexedDB Engine
 *
 * Implements:
 * - Peer-to-peer WebRTC mesh network with dual signaling (SSE Hub + Optical QR codes)
 * - Conflict-free Replicated Data Type (CRDT) monotonic sector claim progression
 * - Local-first offline storage via IndexedDB ('petspotr_mesh_db')
 * - Automatic cloud uplink reconciliation upon network recovery
 *
 * Strict CSP compliance: Zero inline script, zero eval().
 */
(function (global) {
  'use strict';

  // Monotonic CRDT Sector Ranks
  const SectorRanks = {
    UNCLAIMED: 0,
    CLAIMED: 1,
    SEARCHING: 2,
    CLEARED: 3
  };

  function RankFromState(state) {
    if (!state) return 0;
    const upper = String(state).trim().toUpperCase();
    return SectorRanks[upper] !== undefined ? SectorRanks[upper] : 0;
  }

  function StateFromRank(rank) {
    switch (Number(rank)) {
      case 1:
        return 'CLAIMED';
      case 2:
        return 'SEARCHING';
      case 3:
        return 'CLEARED';
      default:
        return 'UNCLAIMED';
    }
  }

  // Safe access to IndexedDB
  function getIndexedDB() {
    if (typeof indexedDB !== 'undefined') return indexedDB;
    if (typeof window !== 'undefined' && window.indexedDB) return window.indexedDB;
    if (typeof globalThis !== 'undefined' && globalThis.indexedDB) return globalThis.indexedDB;
    return null;
  }

  // UUID v4 Generator
  function generateUUID() {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  // CSRF Token reader
  function getCsrfToken() {
    if (typeof window !== 'undefined' && window.petspotrIdentity?.getState) {
      const state = window.petspotrIdentity.getState();
      if (state?.csrfToken) return state.csrfToken;
    }
    if (typeof document !== 'undefined' && document.cookie) {
      const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf=([^;]+)/);
      if (match) return decodeURIComponent(match[1]);
    }
    return '';
  }

  // State
  let currentDBName = 'petspotr_mesh_db';
  let dbInstance = null;
  let isOnline = typeof navigator !== 'undefined' && 'onLine' in navigator ? navigator.onLine : true;
  let currentNodeId = 'node-' + generateUUID().slice(0, 8);
  let currentPartyId = '';
  let currentVolunteer = {
    volunteerId: 'vol-' + generateUUID().slice(0, 6),
    volunteerName: 'Anonymous Searcher',
    role: 'SEARCHER'
  };
  let currentBatteryLevel = 100;
  let lamportClock = 0;
  let breadcrumbSeq = 0;

  // Endpoints
  let sseUrl = '/api/v1/mesh/signal/events';
  let messageUrl = '/api/v1/mesh/signal/message';
  let uplinkUrl = '/api/v1/mesh/uplink-sync';
  let rtcConfig = {
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  };

  // In-memory collections
  const sectorsMap = new Map(); // sectorId -> MeshSectorDelta
  const activePeers = new Map(); // nodeId -> PeerInfo
  const peerConnections = new Map(); // nodeId -> RTCPeerConnection
  const dataChannels = new Map(); // nodeId -> RTCDataChannel
  let pendingOpticalPC = null;
  let eventSource = null;
  let heartbeatTimer = null;
  let syncPromise = null;

  // Dispatch DOM CustomEvent
  function dispatchCustomEvent(name, detail) {
    if (typeof CustomEvent === 'undefined') return;
    const evt = new CustomEvent(name, { detail, bubbles: true });
    if (typeof window !== 'undefined') {
      window.dispatchEvent(evt);
    }
    if (typeof document !== 'undefined') {
      document.dispatchEvent(evt);
    }
  }

  // IndexedDB operations
  function openDB(dbName = currentDBName) {
    if (dbInstance && dbInstance.name === dbName) {
      return Promise.resolve(dbInstance);
    }
    const idb = getIndexedDB();
    if (!idb) return Promise.reject(new Error('IndexedDB not supported'));

    return new Promise((resolve, reject) => {
      const req = idb.open(dbName, 1);
      req.onupgradeneeded = () => {
        const db = req.result;
        if (!db.objectStoreNames.contains('mesh_sectors')) {
          db.createObjectStore('mesh_sectors', { keyPath: 'sectorId' });
        }
        if (!db.objectStoreNames.contains('mesh_breadcrumbs')) {
          const bc = db.createObjectStore('mesh_breadcrumbs', { keyPath: 'id' });
          bc.createIndex('petId', 'petId', { unique: false });
          bc.createIndex('volunteerId', 'volunteerId', { unique: false });
        }
        if (!db.objectStoreNames.contains('mesh_sightings')) {
          db.createObjectStore('mesh_sightings', { keyPath: 'sightingId' });
        }
        if (!db.objectStoreNames.contains('mesh_outbox')) {
          const ob = db.createObjectStore('mesh_outbox', { keyPath: 'id' });
          ob.createIndex('type', 'type', { unique: false });
          ob.createIndex('status', 'status', { unique: false });
        }
      };
      req.onsuccess = () => {
        dbInstance = req.result;
        resolve(dbInstance);
      };
      req.onerror = () => reject(req.error);
    });
  }

  async function putRecord(storeName, record) {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readwrite');
      const store = tx.objectStore(storeName);
      const req = store.put(record);
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  }

  async function getAllRecords(storeName) {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readonly');
      const store = tx.objectStore(storeName);
      const req = store.getAll();
      req.onsuccess = () => resolve(req.result || []);
      req.onerror = () => reject(req.error);
    });
  }

  async function deleteRecord(storeName, key) {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(storeName, 'readwrite');
      const store = tx.objectStore(storeName);
      const req = store.delete(key);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  async function queueOutbox(type, payload) {
    const outboxItem = {
      id: generateUUID(),
      type,
      payload,
      timestamp: new Date().toISOString(),
      status: 'pending'
    };
    await putRecord('mesh_outbox', outboxItem);
    return outboxItem;
  }

  async function getPendingOutbox() {
    return await getAllRecords('mesh_outbox');
  }

  async function loadSectorsFromDB() {
    try {
      const records = await getAllRecords('mesh_sectors');
      for (const rec of records) {
        if (rec && rec.sectorId) {
          sectorsMap.set(rec.sectorId, rec);
        }
      }
    } catch (e) {
      // Ignored if DB empty or initializing
    }
  }

  // CRDT Sector Delta Merge Engine
  async function mergeSectorDelta(delta) {
    if (!delta || !delta.sectorId) return false;

    const rawState = delta.state || StateFromRank(delta.rank);
    const rank = delta.rank !== undefined ? Number(delta.rank) : RankFromState(rawState);
    const incomingClock = Number(delta.lamportClock) || 0;

    const normalizedDelta = {
      petId: delta.petId || currentPartyId || '',
      sectorId: delta.sectorId,
      state: rawState,
      rank: rank,
      claimedByVolunteerId: delta.claimedByVolunteerId || '',
      claimedByName: delta.claimedByName || '',
      nodeId: delta.nodeId || delta.claimedByVolunteerId || '',
      lamportClock: incomingClock,
      timestamp: delta.timestamp || new Date().toISOString()
    };

    if (incomingClock > lamportClock) {
      lamportClock = incomingClock;
    }

    const existing = sectorsMap.get(normalizedDelta.sectorId);
    if (!existing) {
      sectorsMap.set(normalizedDelta.sectorId, normalizedDelta);
      try {
        await putRecord('mesh_sectors', normalizedDelta);
      } catch (e) {}
      dispatchCustomEvent('mesh:sector-updated', normalizedDelta);
      return true;
    }

    // 1. Monotonic Rank Progression: higher rank always wins regardless of Lamport clock
    if (normalizedDelta.rank > existing.rank) {
      sectorsMap.set(normalizedDelta.sectorId, normalizedDelta);
      try {
        await putRecord('mesh_sectors', normalizedDelta);
      } catch (e) {}
      dispatchCustomEvent('mesh:sector-updated', normalizedDelta);
      return true;
    }
    if (normalizedDelta.rank < existing.rank) {
      return false; // Stale lower-rank update discarded
    }

    // 2. Same Rank: Lamport Clock comparison
    if (normalizedDelta.lamportClock > existing.lamportClock) {
      sectorsMap.set(normalizedDelta.sectorId, normalizedDelta);
      try {
        await putRecord('mesh_sectors', normalizedDelta);
      } catch (e) {}
      dispatchCustomEvent('mesh:sector-updated', normalizedDelta);
      return true;
    }
    if (normalizedDelta.lamportClock < existing.lamportClock) {
      return false; // Stale clock discarded
    }

    // 3. Same Rank, Same Clock: Lexicographical NodeID tie-breaker
    const uNode = normalizedDelta.nodeId || normalizedDelta.claimedByVolunteerId;
    const sNode = existing.nodeId || existing.claimedByVolunteerId;
    if (uNode > sNode) {
      sectorsMap.set(normalizedDelta.sectorId, normalizedDelta);
      try {
        await putRecord('mesh_sectors', normalizedDelta);
      } catch (e) {}
      dispatchCustomEvent('mesh:sector-updated', normalizedDelta);
      return true;
    }
    if (uNode < sNode) {
      return false;
    }

    // 4. Equal NodeID: Newer physical timestamp wins
    const dTime = new Date(normalizedDelta.timestamp).getTime();
    const eTime = new Date(existing.timestamp).getTime();
    if (dTime > eTime) {
      sectorsMap.set(normalizedDelta.sectorId, normalizedDelta);
      try {
        await putRecord('mesh_sectors', normalizedDelta);
      } catch (e) {}
      dispatchCustomEvent('mesh:sector-updated', normalizedDelta);
      return true;
    }

    return false;
  }

  // Optical SDP Compression / Decompression
  async function compressSDP(payload) {
    const jsonStr = typeof payload === 'string' ? payload : JSON.stringify(payload);
    if (typeof CompressionStream !== 'undefined') {
      try {
        const stream = new Blob([jsonStr]).stream().pipeThrough(new CompressionStream('deflate-raw'));
        const buffer = await new Response(stream).arrayBuffer();
        const bytes = new Uint8Array(buffer);
        let binary = '';
        for (let i = 0; i < bytes.length; i++) {
          binary += String.fromCharCode(bytes[i]);
        }
        return 'z_' + btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
      } catch (e) {
        // Fallback below
      }
    }
    const encoded = encodeURIComponent(jsonStr).replace(/%([0-9A-F]{2})/g, function (match, p1) {
      return String.fromCharCode(parseInt(p1, 16));
    });
    return 'b_' + btoa(encoded).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }

  async function decompressSDP(str) {
    if (!str) return null;
    const isDeflate = str.startsWith('z_');
    const isBase64 = str.startsWith('b_');
    const rawData = isDeflate || isBase64 ? str.slice(2) : str;
    let b64 = rawData.replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4) b64 += '=';

    if (isDeflate && typeof DecompressionStream !== 'undefined') {
      try {
        const binary = atob(b64);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
        const text = await new Response(stream).text();
        try {
          return JSON.parse(text);
        } catch (e) {
          return text;
        }
      } catch (e) {
        // Fallback below
      }
    }

    try {
      const binary = atob(b64);
      if (typeof DecompressionStream !== 'undefined') {
        try {
          const bytes = new Uint8Array(binary.length);
          for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
          }
          const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
          const text = await new Response(stream).text();
          return JSON.parse(text);
        } catch (e) {}
      }
      const jsonStr = decodeURIComponent(
        Array.prototype.map
          .call(binary, function (c) {
            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
          })
          .join('')
      );
      return JSON.parse(jsonStr);
    } catch (err) {
      return null;
    }
  }

  // Peer & DataChannel Management
  function handlePeerJoined(peerInfo) {
    if (!peerInfo || !peerInfo.nodeId) return;
    const peerRecord = {
      nodeId: peerInfo.nodeId,
      volunteerId: peerInfo.volunteerId || '',
      volunteerName: peerInfo.volunteerName || 'Volunteer',
      role: peerInfo.role || 'SEARCHER',
      batteryLevel: peerInfo.batteryLevel || 100,
      lastSeenAt: new Date()
    };
    activePeers.set(peerInfo.nodeId, peerRecord);
    dispatchCustomEvent('mesh:peer-joined', peerRecord);
  }

  function handlePeerLeft(nodeId) {
    if (!nodeId) return;
    if (activePeers.has(nodeId)) {
      activePeers.delete(nodeId);
      dispatchCustomEvent('mesh:peer-left', { nodeId });
    }
    if (peerConnections.has(nodeId)) {
      try {
        peerConnections.get(nodeId).close();
      } catch (e) {}
      peerConnections.delete(nodeId);
    }
    dataChannels.delete(nodeId);
  }

  function sendDirectMessage(channel, type, payload) {
    if (!channel || channel.readyState !== 'open') return;
    const msg = {
      messageId: generateUUID(),
      searchPartyId: currentPartyId || '',
      senderNodeId: currentNodeId,
      type: type,
      lamportClock: lamportClock,
      timestamp: new Date().toISOString(),
      payload: payload || {}
    };
    try {
      channel.send(JSON.stringify(msg));
    } catch (e) {
      console.warn('Failed to send direct DataChannel message:', e);
    }
  }

  function broadcastDataMessage(type, payload) {
    const msg = {
      messageId: generateUUID(),
      searchPartyId: currentPartyId || '',
      senderNodeId: currentNodeId,
      type: type,
      lamportClock: lamportClock,
      timestamp: new Date().toISOString(),
      payload: payload || {}
    };
    const raw = JSON.stringify(msg);
    for (const [peerNodeId, dc] of dataChannels.entries()) {
      if (dc && dc.readyState === 'open') {
        try {
          dc.send(raw);
        } catch (e) {
          console.warn(`Failed to broadcast to peer ${peerNodeId}:`, e);
        }
      }
    }
  }

  function setupDataChannel(channel, peerNodeId) {
    dataChannels.set(peerNodeId, channel);

    channel.onopen = () => {
      sendDirectMessage(channel, 'JOIN', {
        nodeId: currentNodeId,
        volunteerId: currentVolunteer.volunteerId,
        volunteerName: currentVolunteer.volunteerName,
        role: currentVolunteer.role
      });

      const sectors = Array.from(sectorsMap.values());
      if (sectors.length > 0) {
        sendDirectMessage(channel, 'DELTA_PAYLOAD', { sectors });
      }
    };

    channel.onclose = () => {
      dataChannels.delete(peerNodeId);
      handlePeerLeft(peerNodeId);
    };

    channel.onerror = (err) => {
      console.warn(`DataChannel error with peer ${peerNodeId}:`, err);
    };

    channel.onmessage = async (event) => {
      try {
        const msg = JSON.parse(event.data);
        await handleIncomingDataMessage(msg, peerNodeId);
      } catch (err) {
        console.warn('Failed to parse incoming DataChannel message:', err);
      }
    };
  }

  async function handleIncomingDataMessage(msg, peerNodeId) {
    if (!msg || !msg.type) return;

    if (msg.lamportClock && msg.lamportClock > lamportClock) {
      lamportClock = msg.lamportClock;
    }

    switch (msg.type) {
      case 'JOIN': {
        const pInfo = msg.payload || {};
        handlePeerJoined({
          nodeId: msg.senderNodeId || peerNodeId,
          volunteerId: pInfo.volunteerId || '',
          volunteerName: pInfo.volunteerName || '',
          role: pInfo.role || 'SEARCHER'
        });
        break;
      }
      case 'LEAVE': {
        handlePeerLeft(msg.senderNodeId || peerNodeId);
        break;
      }
      case 'HEARTBEAT': {
        const peer = activePeers.get(msg.senderNodeId || peerNodeId);
        if (peer) {
          peer.lastSeenAt = new Date();
          if (msg.payload?.batteryLevel !== undefined) {
            peer.batteryLevel = msg.payload.batteryLevel;
          }
        }
        break;
      }
      case 'DELTA_PAYLOAD': {
        const payload = msg.payload || {};
        if (Array.isArray(payload.sectors)) {
          for (const s of payload.sectors) {
            await mergeSectorDelta(s);
          }
        }
        if (Array.isArray(payload.breadcrumbs)) {
          for (const bc of payload.breadcrumbs) {
            if (!bc.id) bc.id = `${bc.volunteerId || 'anon'}:${bc.seq || generateUUID()}`;
            try {
              await putRecord('mesh_breadcrumbs', bc);
            } catch (e) {}
            dispatchCustomEvent('mesh:breadcrumb-received', bc);
          }
        }
        if (Array.isArray(payload.sightings)) {
          for (const st of payload.sightings) {
            try {
              await putRecord('mesh_sightings', st);
            } catch (e) {}
            dispatchCustomEvent('mesh:sighting-received', st);
          }
        }
        if (Array.isArray(payload.sosAlerts)) {
          for (const sos of payload.sosAlerts) {
            dispatchCustomEvent('mesh:sos-alert', sos);
          }
        }
        break;
      }
      case 'SOS_ALERT': {
        const alert = msg.payload || msg;
        dispatchCustomEvent('mesh:sos-alert', alert);
        break;
      }
      case 'GOSSIP_DIGEST': {
        // Return latest deltas if requested
        const currentSectors = Array.from(sectorsMap.values());
        if (currentSectors.length > 0) {
          sendDirectMessage(dataChannels.get(peerNodeId), 'DELTA_PAYLOAD', { sectors: currentSectors });
        }
        break;
      }
    }
  }

  function waitForIceGathering(pc) {
    if (pc.iceGatheringState === 'complete') {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      let resolved = false;
      const done = () => {
        if (!resolved) {
          resolved = true;
          pc.removeEventListener('icecandidate', checkCandidate);
          pc.removeEventListener('icegatheringstatechange', checkState);
          resolve();
        }
      };
      const checkCandidate = (e) => {
        if (!e.candidate) done();
      };
      const checkState = () => {
        if (pc.iceGatheringState === 'complete') done();
      };
      pc.addEventListener('icecandidate', checkCandidate);
      pc.addEventListener('icegatheringstatechange', checkState);
      setTimeout(done, 1500);
    });
  }

  // Mode 1: HTTP / SSE Signaling
  async function postSignalingEnvelope(envelope) {
    if (!isOnline) return;
    const headers = { 'Content-Type': 'application/json' };
    const csrf = getCsrfToken();
    if (csrf) headers['X-CSRF-Token'] = csrf;

    try {
      return await fetch(messageUrl, {
        method: 'POST',
        headers,
        body: JSON.stringify(envelope)
      });
    } catch (e) {
      console.warn('Failed to post signaling envelope:', e);
    }
  }

  async function initiatePeerConnection(targetNodeId) {
    if (typeof RTCPeerConnection === 'undefined') return;
    if (peerConnections.has(targetNodeId)) return;

    const pc = new RTCPeerConnection(rtcConfig);
    peerConnections.set(targetNodeId, pc);

    const dc = pc.createDataChannel('mesh-sync-channel', { ordered: true });
    setupDataChannel(dc, targetNodeId);

    pc.onicecandidate = (e) => {
      if (e.candidate) {
        postSignalingEnvelope({
          type: 'ice_candidate',
          searchPartyId: currentPartyId,
          senderNodeId: currentNodeId,
          targetNodeId: targetNodeId,
          payload: { candidate: e.candidate.candidate, sdpMid: e.candidate.sdpMid, sdpMLineIndex: e.candidate.sdpMLineIndex }
        });
      }
    };

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    await postSignalingEnvelope({
      type: 'offer',
      searchPartyId: currentPartyId,
      senderNodeId: currentNodeId,
      targetNodeId: targetNodeId,
      payload: { sdp: offer.sdp, type: offer.type }
    });
  }

  async function handleIncomingOffer(envelope) {
    if (typeof RTCPeerConnection === 'undefined') return;
    const senderNodeId = envelope.senderNodeId;
    if (!senderNodeId) return;

    const pc = new RTCPeerConnection(rtcConfig);
    peerConnections.set(senderNodeId, pc);

    pc.ondatachannel = (e) => {
      setupDataChannel(e.channel, senderNodeId);
    };

    pc.onicecandidate = (e) => {
      if (e.candidate) {
        postSignalingEnvelope({
          type: 'ice_candidate',
          searchPartyId: currentPartyId,
          senderNodeId: currentNodeId,
          targetNodeId: senderNodeId,
          payload: { candidate: e.candidate.candidate, sdpMid: e.candidate.sdpMid, sdpMLineIndex: e.candidate.sdpMLineIndex }
        });
      }
    };

    const offerPayload = envelope.payload || {};
    const sdpVal = offerPayload.sdp || envelope.sdp;
    await pc.setRemoteDescription(new RTCSessionDescription({
      type: offerPayload.type || envelope.type || 'offer',
      sdp: sdpVal
    }));

    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);

    await postSignalingEnvelope({
      type: 'answer',
      searchPartyId: currentPartyId,
      senderNodeId: currentNodeId,
      targetNodeId: senderNodeId,
      payload: { sdp: answer.sdp, type: answer.type }
    });
  }

  async function handleIncomingAnswer(envelope) {
    const pc = peerConnections.get(envelope.senderNodeId);
    if (!pc) return;
    const answerPayload = envelope.payload || {};
    const sdpVal = answerPayload.sdp || envelope.sdp;
    await pc.setRemoteDescription(new RTCSessionDescription({
      type: answerPayload.type || envelope.type || 'answer',
      sdp: sdpVal
    }));
  }

  async function handleIncomingCandidate(envelope) {
    const pc = peerConnections.get(envelope.senderNodeId);
    if (!pc) return;
    const cPayload = envelope.payload || {};
    const cand = cPayload.candidate || envelope.candidate;
    if (cand) {
      try {
        await pc.addIceCandidate(new RTCIceCandidate({
          candidate: cand,
          sdpMid: cPayload.sdpMid || envelope.sdpMid,
          sdpMLineIndex: cPayload.sdpMLineIndex ?? envelope.sdpMLineIndex
        }));
      } catch (e) {
        console.warn('Failed to add ICE candidate:', e);
      }
    }
  }

  function connectSignaling() {
    if (typeof EventSource === 'undefined') return;
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    if (!currentPartyId || !currentNodeId) return;

    const url = `${sseUrl}?searchPartyId=${encodeURIComponent(currentPartyId)}&nodeId=${encodeURIComponent(currentNodeId)}`;
    eventSource = new EventSource(url);

    const onOffer = (e) => {
      try {
        const envelope = JSON.parse(e.data);
        if (envelope.targetNodeId && envelope.targetNodeId !== currentNodeId) return;
        handleIncomingOffer(envelope).catch(() => {});
      } catch (err) {}
    };
    eventSource.addEventListener('offer', onOffer);
    eventSource.addEventListener('OFFER', onOffer);

    const onAnswer = (e) => {
      try {
        const envelope = JSON.parse(e.data);
        if (envelope.targetNodeId && envelope.targetNodeId !== currentNodeId) return;
        handleIncomingAnswer(envelope).catch(() => {});
      } catch (err) {}
    };
    eventSource.addEventListener('answer', onAnswer);
    eventSource.addEventListener('ANSWER', onAnswer);

    const onCandidate = (e) => {
      try {
        const envelope = JSON.parse(e.data);
        if (envelope.targetNodeId && envelope.targetNodeId !== currentNodeId) return;
        handleIncomingCandidate(envelope).catch(() => {});
      } catch (err) {}
    };
    eventSource.addEventListener('ice_candidate', onCandidate);
    eventSource.addEventListener('ICE_CANDIDATE', onCandidate);

    const onJoin = (e) => {
      try {
        const envelope = JSON.parse(e.data);
        if (envelope.senderNodeId && envelope.senderNodeId !== currentNodeId) {
          if (currentNodeId > envelope.senderNodeId) {
            initiatePeerConnection(envelope.senderNodeId).catch(() => {});
          }
        }
      } catch (err) {}
    };
    eventSource.addEventListener('join', onJoin);
    eventSource.addEventListener('PEER_JOINED', onJoin);

    const onLeave = (e) => {
      try {
        const envelope = JSON.parse(e.data);
        handlePeerLeft(envelope.senderNodeId);
      } catch (err) {}
    };
    eventSource.addEventListener('leave', onLeave);
    eventSource.addEventListener('PEER_LEFT', onLeave);

    // Announce join
    postSignalingEnvelope({
      type: 'join',
      searchPartyId: currentPartyId,
      senderNodeId: currentNodeId,
      payload: {
        volunteerId: currentVolunteer.volunteerId,
        volunteerName: currentVolunteer.volunteerName,
        role: currentVolunteer.role
      }
    }).catch(() => {});
  }

  function startHeartbeat() {
    if (heartbeatTimer) clearInterval(heartbeatTimer);
    heartbeatTimer = setInterval(() => {
      broadcastDataMessage('HEARTBEAT', {
        batteryLevel: currentBatteryLevel
      });
    }, 3000);
  }

  // Cloud WAN Uplink Synchronization
  async function syncUplink() {
    if (syncPromise) return syncPromise;
    syncPromise = (async () => {
      try {
        const pendingItems = await getPendingOutbox();
        if (!pendingItems || pendingItems.length === 0) {
          return { success: true, count: 0 };
        }

        const batch = {
          searchPartyId: currentPartyId || '',
          senderNodeId: currentNodeId,
          sectors: [],
          breadcrumbs: [],
          sightings: [],
          evacuations: [],
          sosAlerts: []
        };

        for (const item of pendingItems) {
          if (item.type === 'sector') batch.sectors.push(item.payload);
          else if (item.type === 'breadcrumb') batch.breadcrumbs.push(item.payload);
          else if (item.type === 'sighting') batch.sightings.push(item.payload);
          else if (item.type === 'evacuation') batch.evacuations.push(item.payload);
          else if (item.type === 'sos') batch.sosAlerts.push(item.payload);
        }

        const headers = { 'Content-Type': 'application/json' };
        const csrf = getCsrfToken();
        if (csrf) headers['X-CSRF-Token'] = csrf;

        const resp = await fetch(uplinkUrl, {
          method: 'POST',
          headers,
          body: JSON.stringify(batch)
        });

        if (!resp.ok) {
          throw new Error(`Uplink sync failed: HTTP ${resp.status}`);
        }

        const data = await resp.json();

        // Clear synced items from outbox
        for (const item of pendingItems) {
          await deleteRecord('mesh_outbox', item.id);
        }

        // Reconcile serverLatestDeltas if any returned
        if (data.serverLatestDeltas) {
          if (Array.isArray(data.serverLatestDeltas.sectors)) {
            for (const s of data.serverLatestDeltas.sectors) {
              await mergeSectorDelta(s);
            }
          }
          if (Array.isArray(data.serverLatestDeltas.breadcrumbs)) {
            for (const bc of data.serverLatestDeltas.breadcrumbs) {
              if (!bc.id) bc.id = `${bc.volunteerId || 'anon'}:${bc.seq || generateUUID()}`;
              try {
                await putRecord('mesh_breadcrumbs', bc);
              } catch (e) {}
              dispatchCustomEvent('mesh:breadcrumb-received', bc);
            }
          }
        }

        dispatchCustomEvent('mesh:uplink-complete', {
          result: data,
          count: pendingItems.length
        });

        return data;
      } finally {
        syncPromise = null;
      }
    })();
    return syncPromise;
  }

  // Public Interface
  const PetSpotRMesh = {
    RankFromState,
    StateFromRank,
    compressSDP,
    decompressSDP,

    async init(options = {}) {
      if (options.dbName) currentDBName = options.dbName;
      if (options.partyId) currentPartyId = options.partyId;
      if (options.sseUrl) sseUrl = options.sseUrl;
      if (options.messageUrl) messageUrl = options.messageUrl;
      if (options.uplinkUrl) uplinkUrl = options.uplinkUrl;
      if (options.rtcConfig) rtcConfig = options.rtcConfig;

      if (options.volunteerInfo) {
        if (options.volunteerInfo.volunteerId) currentVolunteer.volunteerId = options.volunteerInfo.volunteerId;
        if (options.volunteerInfo.volunteerName) currentVolunteer.volunteerName = options.volunteerInfo.volunteerName;
        if (options.volunteerInfo.role) currentVolunteer.role = options.volunteerInfo.role;
        if (options.volunteerInfo.nodeId) currentNodeId = options.volunteerInfo.nodeId;
      }

      await openDB(currentDBName);
      await loadSectorsFromDB();

      startHeartbeat();

      if (isOnline && currentPartyId) {
        connectSignaling();
      }

      return this.getState();
    },

    async joinSearchParty(partyId, volunteerInfo = {}) {
      currentPartyId = partyId;
      if (volunteerInfo.volunteerId) currentVolunteer.volunteerId = volunteerInfo.volunteerId;
      if (volunteerInfo.volunteerName) currentVolunteer.volunteerName = volunteerInfo.volunteerName;
      if (volunteerInfo.role) currentVolunteer.role = volunteerInfo.role;
      if (volunteerInfo.nodeId) currentNodeId = volunteerInfo.nodeId;

      await openDB(currentDBName);
      await loadSectorsFromDB();

      if (isOnline) {
        connectSignaling();
      }

      return this.getState();
    },

    async claimSector(petId, sectorId, state = 'CLAIMED', options = {}) {
      const rank = RankFromState(state);
      lamportClock++;
      const delta = {
        petId: petId || currentPartyId || '',
        sectorId: sectorId,
        state: state,
        rank: rank,
        claimedByVolunteerId: options.volunteerId || currentVolunteer.volunteerId || '',
        claimedByName: options.claimedByName || options.volunteerName || currentVolunteer.volunteerName || '',
        nodeId: currentNodeId,
        lamportClock: lamportClock,
        timestamp: new Date().toISOString()
      };

      const accepted = await mergeSectorDelta(delta);
      if (accepted) {
        await queueOutbox('sector', delta);
        broadcastDataMessage('DELTA_PAYLOAD', { sectors: [delta] });
      }
      return accepted;
    },

    async recordBreadcrumb(petId, lat, lng, options = {}) {
      breadcrumbSeq++;
      const delta = {
        id: generateUUID(),
        petId: petId || currentPartyId || '',
        volunteerId: currentVolunteer.volunteerId || '',
        volunteerName: currentVolunteer.volunteerName || '',
        seq: breadcrumbSeq,
        latitude: Number(lat),
        longitude: Number(lng),
        accuracy: options.accuracy || 5.0,
        timestamp: options.timestamp || new Date().toISOString()
      };

      try {
        await putRecord('mesh_breadcrumbs', delta);
      } catch (e) {}
      await queueOutbox('breadcrumb', delta);
      broadcastDataMessage('DELTA_PAYLOAD', { breadcrumbs: [delta] });
      dispatchCustomEvent('mesh:breadcrumb-received', delta);
      return delta;
    },

    async broadcastSOS(message, coords = {}) {
      lamportClock++;
      const alert = {
        alertId: generateUUID(),
        volunteerId: currentVolunteer.volunteerId || '',
        volunteerName: currentVolunteer.volunteerName || '',
        latitude: coords.latitude || 0,
        longitude: coords.longitude || 0,
        message: String(message || 'Distress SOS Alert'),
        timestamp: new Date().toISOString()
      };

      broadcastDataMessage('SOS_ALERT', alert);
      dispatchCustomEvent('mesh:sos-alert', alert);
      try {
        await queueOutbox('sos', alert);
      } catch (e) {}
      return alert;
    },

    async generateOpticalOffer() {
      if (typeof RTCPeerConnection === 'undefined') {
        throw new Error('WebRTC RTCPeerConnection is not supported in this environment');
      }
      const pc = new RTCPeerConnection(rtcConfig);
      pendingOpticalPC = pc;

      const dc = pc.createDataChannel('mesh-sync-channel', { ordered: true });
      setupDataChannel(dc, 'optical-responder');

      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);

      await waitForIceGathering(pc);

      const payload = {
        type: 'offer',
        sdp: pc.localDescription ? pc.localDescription.sdp : offer.sdp,
        nodeId: currentNodeId,
        searchPartyId: currentPartyId,
        volunteerId: currentVolunteer.volunteerId,
        volunteerName: currentVolunteer.volunteerName,
        role: currentVolunteer.role
      };

      return await compressSDP(payload);
    },

    async generateOpticalAnswer(offerStr) {
      if (typeof RTCPeerConnection === 'undefined') {
        throw new Error('WebRTC RTCPeerConnection is not supported in this environment');
      }
      const offerPayload = await decompressSDP(offerStr);
      if (!offerPayload || !offerPayload.sdp) {
        throw new Error('Invalid optical offer payload');
      }

      const pc = new RTCPeerConnection(rtcConfig);
      pc.ondatachannel = (e) => {
        setupDataChannel(e.channel, offerPayload.nodeId || 'optical-initiator');
      };

      await pc.setRemoteDescription(new RTCSessionDescription({
        type: 'offer',
        sdp: offerPayload.sdp
      }));

      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);

      await waitForIceGathering(pc);

      if (offerPayload.nodeId) {
        peerConnections.set(offerPayload.nodeId, pc);
        handlePeerJoined({
          nodeId: offerPayload.nodeId,
          volunteerId: offerPayload.volunteerId,
          volunteerName: offerPayload.volunteerName,
          role: offerPayload.role
        });
      }

      const payload = {
        type: 'answer',
        sdp: pc.localDescription ? pc.localDescription.sdp : answer.sdp,
        nodeId: currentNodeId,
        searchPartyId: currentPartyId,
        volunteerId: currentVolunteer.volunteerId,
        volunteerName: currentVolunteer.volunteerName,
        role: currentVolunteer.role
      };

      return await compressSDP(payload);
    },

    async consumeOpticalAnswer(answerStr) {
      const answerPayload = await decompressSDP(answerStr);
      if (!answerPayload || !answerPayload.sdp) {
        throw new Error('Invalid optical answer payload');
      }

      if (!pendingOpticalPC) {
        throw new Error('No pending optical peer connection');
      }

      await pendingOpticalPC.setRemoteDescription(new RTCSessionDescription({
        type: 'answer',
        sdp: answerPayload.sdp
      }));

      if (answerPayload.nodeId) {
        peerConnections.set(answerPayload.nodeId, pendingOpticalPC);
        handlePeerJoined({
          nodeId: answerPayload.nodeId,
          volunteerId: answerPayload.volunteerId,
          volunteerName: answerPayload.volunteerName,
          role: answerPayload.role
        });
      }

      const pc = pendingOpticalPC;
      pendingOpticalPC = null;
      return { success: true, peerConnection: pc };
    },

    syncUplink,
    mergeSectorDelta,
    getPendingOutbox,

    getSector(sectorId) {
      return sectorsMap.get(sectorId) || null;
    },

    handlePeerJoined,
    handlePeerLeft,

    getPeers() {
      return Array.from(activePeers.values());
    },

    setOnlineStatus(online) {
      isOnline = Boolean(online);
    },

    getState() {
      return {
        nodeId: currentNodeId,
        partyId: currentPartyId,
        volunteer: { ...currentVolunteer },
        lamportClock,
        isOnline,
        activePeers: Array.from(activePeers.values()),
        sectors: Array.from(sectorsMap.values()),
        peerCount: activePeers.size
      };
    },

    close() {
      if (heartbeatTimer) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = null;
      }
      if (eventSource) {
        eventSource.close();
        eventSource = null;
      }
      for (const dc of dataChannels.values()) {
        try {
          dc.close();
        } catch (e) {}
      }
      dataChannels.clear();
      for (const pc of peerConnections.values()) {
        try {
          pc.close();
        } catch (e) {}
      }
      peerConnections.clear();
      if (pendingOpticalPC) {
        try {
          pendingOpticalPC.close();
        } catch (e) {}
        pendingOpticalPC = null;
      }
      activePeers.clear();
      if (dbInstance) {
        try {
          dbInstance.close();
        } catch (e) {}
        dbInstance = null;
      }
    }
  };

  // Setup browser environment listeners
  if (typeof window !== 'undefined') {
    window.petSpotRMesh = PetSpotRMesh;

    window.addEventListener('online', () => {
      isOnline = true;
      PetSpotRMesh.syncUplink().catch(() => {});
    });

    window.addEventListener('offline', () => {
      isOnline = false;
    });
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = PetSpotRMesh;
  }
})(typeof window !== 'undefined' ? window : globalThis);
