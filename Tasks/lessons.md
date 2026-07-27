# Lessons

## 2026-05-12

- For VR filtering UX, do not default to per-interaction server recomputation
  when the same intersection can be derived from a compact one-time index.
  Headset latency is a product requirement, not an implementation detail.
- When extending an existing curved VR surface, reuse its shared cylinder hub,
  radius, and seam coordinates. A standalone plane may be functionally valid
  but visually disconnects the new UI from the interaction it is meant to
  extend.
- For dynamically generated VR canvas labels, create the textured Three.js
  mesh directly instead of relying on an A-Frame child plane's asynchronous
  `loaded` callback. Until that callback runs, the default white plane can
  obscure the whole UI and intercept intended controls.
- A visual match is not a feature match: when reproducing a desktop panel in
  VR, implement each visible control's underlying state transition and data
  action before calling it complete. Curvature must apply to the interaction
  surface as well as the background; tangent planes are not an acceptable
  substitute for a curved row.
- When a curved VR label is rendered from the cylinder's interior, explicitly
  validate UV orientation and depth separation. Double-sided geometry can
  mirror canvas text, and millimeter-scale offsets can still z-fight against
  faceted A-Frame primitives; test both from the headset view.
- If a user asks to clone a reference UI's interactable elements, treat the
  whole referenced surface as the scope. Do not implement a representative
  subset of controls or assume a read-only tab is complete without first
  inventorying the source interactions and their data mutations.
