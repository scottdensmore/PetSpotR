// Infer the browser hooks from their implementations; this file emits no code.
export {};

declare global {
  interface Window {
    petSpotRMesh: typeof import('../../internal/app/webfrontend/static/js/mesh-sync.js');
    PetSpotROutbox: typeof import('../../internal/app/webfrontend/static/js/outbox-sync.js');
  }
}
