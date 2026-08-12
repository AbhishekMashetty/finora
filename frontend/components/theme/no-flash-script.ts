/**
 * Runs synchronously before hydration to apply a persisted theme choice
 * before first paint. Kept as a plain string (not a component) because it
 * has to execute as an inline <script> in app/layout.tsx, outside React's
 * render cycle. Mirrors ThemeProvider's storage key and attribute contract
 * — the two must stay in sync by hand, there's no shared import path across
 * the server/inline-script boundary.
 */
export const noFlashThemeScript = `(function(){try{var t=localStorage.getItem('finora-theme');if(t==='light'||t==='dark'){document.documentElement.setAttribute('data-theme',t);}}catch(e){}})();`;
