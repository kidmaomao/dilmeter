import fs from "node:fs";

const source = fs.readFileSync(new URL("../src/main.ts", import.meta.url), "utf8");
const mountAt = source.indexOf('app.use(vuetify).mount("#app")');
const readyAt = source.indexOf("__dilmeterMainReady");

if (mountAt < 0 || readyAt < 0 || readyAt < mountAt) {
    throw new Error("native-ready signal must run after Vue mounts");
}

const host = fs.readFileSync(new URL("../../cmd/dilmeterapi/main_windows.go", import.meta.url), "utf8");
const bindAt = host.indexOf('view.Bind("__dilmeterMainReady"');
const navigateAt = host.indexOf("view.Navigate(fmt.Sprintf");

if (bindAt < 0 || navigateAt < 0 || bindAt > navigateAt) {
    throw new Error("native-ready binding must be installed before main navigation");
}
if (host.includes("function notifyReady()") || host.includes('document.addEventListener("DOMContentLoaded"', bindAt)) {
    throw new Error("host must not infer Vue readiness from DOMContentLoaded");
}

console.log("native-ready mount verification passed");
