// Named exports that other modules import directly. Page COMPONENTS are NOT
// listed here — they are discovered from the filesystem by convention (see
// Config.loadBarrel): any PageComponent subclass exported from
// pages/<dir>/<Name>.tsx (or a flat pages/<Name>.tsx) is picked up automatically.
// A backend page with an endpoint just needs a matching component; a
// pure-frontend page needs only isPage = true. Nothing is hand-registered.
export { default as Home } from "./home/Home"
export { default as Fieldset } from "./Fieldset"
export { default as ErrorBoundary } from "./ErrorBoundary"
