/* Native JSON exists on IE8+ with json2.min.js polyfill for older caches.
   Keep this file as a safe no-op when JSON is already present. */
(function (global) {
  if (global.JSON && typeof global.JSON.parse === 'function' &&
      typeof global.JSON.stringify === 'function') {
    return;
  }
  // json2.min.js should have been loaded via <!--[if lt IE 9]> for IE8.
  if (typeof console !== 'undefined' && console.warn) {
    console.warn('[legacy-embed] JSON polyfill missing');
  }
})(typeof window !== 'undefined' ? window : this);
