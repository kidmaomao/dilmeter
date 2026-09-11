import { createApp } from "vue";
import App from "@/App.vue";
import * as store from "@/store";
// mdi
import "@mdi/font/css/materialdesignicons.css";

// vuetify
import "vuetify/styles";
import "@/uiColorTheme.css";
import { createVuetify } from "vuetify";

const vuetify = createVuetify({
    theme: {
        defaultTheme: "dark",
    },
});

const app = createApp(App);

// register global variables
for (const _key in store) {
    const key = _key as keyof typeof store;
    app.provide(key, store[key]);
}

app.config.errorHandler = (err) => {
    alert(err);
    console.error(err);
};

app.use(vuetify).mount("#app");

// The native host keeps its window hidden until Vue has actually mounted. A
// DOMContentLoaded signal is too early for module scripts and could expose an
// empty WebView during an automatic-update restart.
const nativeReady = (window as typeof window & {
    __dilmeterMainReady?: () => Promise<unknown>;
}).__dilmeterMainReady;
if (typeof nativeReady === "function") {
    void nativeReady();
}
