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
      this.sceneEl = this.el.sceneEl || this.el;
      this.tmpVec = new AFRAME.THREE.Vector3();
      this.startPos = new AFRAME.THREE.Vector3();
      this.curPos = new AFRAME.THREE.Vector3();
      this.deltaPos = new AFRAME.THREE.Vector3();

      // Per-hand trigger state. handId ∈ {'right','left'}.
      // phase ∈ {'idle','candidate','drag'}.
      this.triggerState = {
        right: this._newTriggerState(),
        left:  this._newTriggerState()
      };

      // Drag thresholds (spec §4.1).
      this.DRAG_DIST_M = 0.05; // 5 cm
      this.DRAG_HOLD_MS = 250;

      this._onTriggerDown = this._onTriggerDown.bind(this);
      this._onTriggerUp   = this._onTriggerUp.bind(this);

      // Find both laser-controls entities (they emit triggerdown/up).
      this._lasers = {
        right: document.getElementById('vrLaserRight'),
        left:  document.getElementById('vrLaserLeft')
      };
      Object.keys(this._lasers).forEach(hand => {
        const el = this._lasers[hand];
        if (!el) return;
        el.addEventListener('triggerdown', e => this._onTriggerDown(hand, e));
        el.addEventListener('triggerup',   e => this._onTriggerUp(hand, e));
      });
    },

    _newTriggerState: function() {
      return {
        phase: 'idle',
        downTime: 0,
        startPos: new AFRAME.THREE.Vector3(),
        hadIntersection: false
      };
    },

    _getControllerPos: function(hand, out) {
      const el = this._lasers[hand];
      if (!el || !el.object3D) { out.set(0, 0, 0); return false; }
      el.object3D.getWorldPosition(out);
      return true;
    },

    _hasRaycastIntersection: function(hand) {
      const el = this._lasers[hand];
      if (!el) return false;
      const rc = el.components && el.components.raycaster;
      if (!rc) return false;
      const its = rc.intersections || [];
      return its.length > 0;
    },

    _onTriggerDown: function(hand) {
      const st = this.triggerState[hand];
      st.phase = 'candidate';
      st.downTime = performance.now();
      this._getControllerPos(hand, st.startPos);
      st.hadIntersection = this._hasRaycastIntersection(hand);
    },

    _onTriggerUp: function(hand) {
      const st = this.triggerState[hand];
      if (st.phase === 'drag') {
        this.sceneEl.emit('m3c:drag-end', { hand: hand });
      }
      // 'candidate' resolution is handled by Task 5; for now just reset.
      st.phase = 'idle';
    },

    tick: function(time, delta) {
      // Promote candidate → drag if either threshold crosses.
      ['right', 'left'].forEach(hand => {
        const st = this.triggerState[hand];
        if (st.phase === 'candidate') {
          const elapsed = performance.now() - st.downTime;
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          const dist = this.deltaPos.length();
          if (dist > this.DRAG_DIST_M || elapsed > this.DRAG_HOLD_MS) {
            st.phase = 'drag';
            this.sceneEl.emit('m3c:drag-start', { hand: hand });
          }
        }
        if (st.phase === 'drag') {
          this._getControllerPos(hand, this.curPos);
          this.deltaPos.subVectors(this.curPos, st.startPos);
          this.sceneEl.emit('m3c:drag-move', {
            hand: hand,
            dx: this.deltaPos.x,
            dy: this.deltaPos.y,
            dz: this.deltaPos.z
          });
          // Reset startPos to curPos so next tick's delta is incremental,
          // not cumulative. This makes the geometry follow the controller
          // 1:1 instead of accelerating.
          st.startPos.copy(this.curPos);
        }
      });
    },

    remove: function() {
      // Listeners attached to laser-controls entities are torn down by
      // their entity removal; nothing global to clean.
    }
  });
})();
