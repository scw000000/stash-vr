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
      this.DOUBLE_CLICK_MS = 400;
      this._pendingClick = null;

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

      // A (right) and X (left) mirror trigger clicks — single → panel toggle,
      // double → play/pause. No raycast branch, no drag (only the trigger
      // captures pose during hold).
      this._onAxClickUp = this._onAxClickUp.bind(this);
      const rightLaser = this._lasers.right;
      const leftLaser  = this._lasers.left;
      if (rightLaser) {
        rightLaser.addEventListener('abuttonup', this._onAxClickUp);
      }
      if (leftLaser) {
        leftLaser.addEventListener('xbuttonup', this._onAxClickUp);
      }

      // B (right) / Y (left) — short-press = reset, long-press = recenter (Task 8).
      this.LONG_PRESS_MS = 500;
      this._byState = {
        b: { downTime: 0, fired: false },
        y: { downTime: 0, fired: false }
      };
      this._onByDown = this._onByDown.bind(this);
      this._onByUp   = this._onByUp.bind(this);
      if (rightLaser) {
        rightLaser.addEventListener('bbuttondown', () => this._onByDown('b'));
        rightLaser.addEventListener('bbuttonup',   () => this._onByUp('b'));
      }
      if (leftLaser) {
        leftLaser.addEventListener('ybuttondown', () => this._onByDown('y'));
        leftLaser.addEventListener('ybuttonup',   () => this._onByUp('y'));
      }

      // Thumbstick state per hand. seekArmed=true means the next
      // |axis_x|>0.7 will fire a seek event.
      this.SEEK_TRIGGER = 0.7;
      this.SEEK_REARM   = 0.3;
      this.SEEK_SECONDS = 10;
      this.thumbState = {
        right: { seekArmed: true },
        left:  { seekArmed: true }
      };
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
        st.phase = 'idle';
        return;
      }
      if (st.phase !== 'candidate') {
        st.phase = 'idle';
        return;
      }
      // Candidate: triggerup before drag thresholds.
      // If it had a raycast intersection, A-Frame's click pipeline
      // already fires the button — do nothing extra.
      if (st.hadIntersection) {
        st.phase = 'idle';
        return;
      }
      // Otherwise: it's a click candidate. Defer for double-click window.
      const now = performance.now();
      if (this._pendingClick && (now - this._pendingClick.time) <= this.DOUBLE_CLICK_MS) {
        // Second click within window → double-click.
        clearTimeout(this._pendingClick.timer);
        this._pendingClick = null;
        this.sceneEl.emit('m3c:play-pause');
      } else {
        // Start a new pending; resolve as single-click after window.
        if (this._pendingClick) clearTimeout(this._pendingClick.timer);
        const pending = { time: now, timer: 0 };
        pending.timer = setTimeout(() => {
          if (this._pendingClick === pending) {
            this._pendingClick = null;
            this.sceneEl.emit('m3c:panel-toggle');
          }
        }, this.DOUBLE_CLICK_MS);
        this._pendingClick = pending;
      }
      st.phase = 'idle';
    },

    _onAxClickUp: function() {
      // Same double-click window as the trigger. No raycast / no drag.
      const now = performance.now();
      if (this._pendingClick && (now - this._pendingClick.time) <= this.DOUBLE_CLICK_MS) {
        clearTimeout(this._pendingClick.timer);
        this._pendingClick = null;
        this.sceneEl.emit('m3c:play-pause');
        return;
      }
      if (this._pendingClick) clearTimeout(this._pendingClick.timer);
      const pending = { time: now, timer: 0 };
      pending.timer = setTimeout(() => {
        if (this._pendingClick === pending) {
          this._pendingClick = null;
          this.sceneEl.emit('m3c:panel-toggle');
        }
      }, this.DOUBLE_CLICK_MS);
      this._pendingClick = pending;
    },

    _onByDown: function(which) {
      const st = this._byState[which];
      st.downTime = performance.now();
      st.fired = false;
    },
    _onByUp: function(which) {
      const st = this._byState[which];
      const wasFired = st.fired;
      const pressTime = st.downTime;  // capture before reset; downTime=0 means no prior press.
      st.downTime = 0;
      st.fired = false;
      if (wasFired) return; // long-press already fired
      if (pressTime !== 0 && performance.now() - pressTime < this.LONG_PRESS_MS) {
        const mode = this._currentMode();
        this.sceneEl.emit('m3c:reset-short', { mode: mode });
      }
    },
    _currentMode: function() {
      // Cinema = the flat plane is the active geometry. Otherwise immersive.
      const flat = document.getElementById('vrFlat');
      if (!flat) return 'immersive';
      const v = flat.getAttribute('visible');
      return (v === true || v === 'true') ? 'cinema' : 'immersive';
    },

    _getThumbstick: function(hand) {
      const el = this._lasers[hand];
      if (!el) return null;
      const tc = el.components && el.components['tracked-controls'];
      const ctrl = tc && tc.controller;
      const axes = ctrl && ctrl.axes;
      if (!axes || axes.length < 2) return null;
      // xr-standard mapping puts thumbstick on axes[2],[3]; legacy on [0],[1].
      // Prefer the pair with larger magnitude (likely non-default).
      let x = 0, y = 0;
      if (axes.length >= 4) {
        x = axes[2] || 0;
        y = axes[3] || 0;
        if (Math.abs(x) < 0.001 && Math.abs(y) < 0.001) {
          x = axes[0] || 0;
          y = axes[1] || 0;
        }
      } else {
        x = axes[0] || 0;
        y = axes[1] || 0;
      }
      return { x: x, y: y };
    },

    tick: function(time, delta) {
      // Trigger candidate → drag promotion (existing).
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
          st.startPos.copy(this.curPos);
        }
      });
      // B/Y long-press promotion.
      ['b', 'y'].forEach(which => {
        const st = this._byState[which];
        if (st.downTime === 0 || st.fired) return;
        if (performance.now() - st.downTime >= this.LONG_PRESS_MS) {
          st.fired = true;
          const mode = this._currentMode();
          this.sceneEl.emit('m3c:reset-long', { mode: mode });
        }
      });

      // Thumbstick X — discrete seek with arm/rearm.
      ['right', 'left'].forEach(hand => {
        const ts = this._getThumbstick(hand);
        if (!ts) return;
        const st = this.thumbState[hand];
        const ax = Math.abs(ts.x);
        if (st.seekArmed && ax > this.SEEK_TRIGGER) {
          const sign = ts.x > 0 ? 1 : -1;
          this.sceneEl.emit('m3c:seek', { sign: sign, seconds: this.SEEK_SECONDS });
          st.seekArmed = false;
        } else if (!st.seekArmed && ax < this.SEEK_REARM) {
          st.seekArmed = true;
        }
      });

      // Thumbstick Y — continuous scale of active geometry.
      // xr-standard: stick up reads as negative axis_y. Invert so positive = up = scale up.
      const dtSec = (delta || 0) / 1000;
      ['right', 'left'].forEach(hand => {
        const ts = this._getThumbstick(hand);
        if (!ts) return;
        const yNorm = -ts.y; // up = positive
        if (Math.abs(yNorm) > 0.3 && dtSec > 0) {
          const factor = 1 + 0.6 * yNorm * dtSec;
          this.sceneEl.emit('m3c:scale', { factor: factor });
        }
      });
    },

    remove: function() {
      // Cancel any pending single-click timer so it doesn't fire after
      // the component is gone (would call this.sceneEl.emit on a
      // possibly-torn-down scene).
      if (this._pendingClick) {
        clearTimeout(this._pendingClick.timer);
        this._pendingClick = null;
      }
      // Listeners attached to laser-controls entities are torn down by
      // their entity removal.
    }
  });
})();
