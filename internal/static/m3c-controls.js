/* M3c controller mappings — see docs/superpowers/specs/2026-05-08-m3c-skybox-controller-mappings.md
 *
 * This component listens for controller input on <a-scene> and emits
 * semantic events (m3c:panel-toggle, m3c:play-pause, m3c:seek, m3c:scale,
 * m3c:drag-start/move/end, m3c:reset-short, m3c:reset-long). The inline
 * IIFE in browse_scene.gohtml owns DOM/video state mutation in response.
 */
(function() {
  if (typeof AFRAME === 'undefined') return;

  AFRAME.registerComponent('m3c-controls', {
    init: function() {
      // Filled in by later tasks. For now just confirm registration.
      console.log('m3c-controls: init');
    },
    tick: function(time, delta) {
      // Filled in by later tasks (gamepad polling).
    },
    remove: function() {
      // Filled in by later tasks (event-listener teardown).
    }
  });
})();
