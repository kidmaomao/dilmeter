import { createApp } from "vue";
import App from "@/App.vue";
import * as store from "@/store";
import { uiLocale, uiText, uiItems } from "@/uiLocale";
import { jobDisplayName } from "@/gameNameDisplay";
import { watch } from "vue";
import { zhHans, zhHant } from "vuetify/locale";
// mdi
import "@mdi/font/css/materialdesignicons.css";

// vuetify
import "vuetify/styles";
import "@/uiColorTheme.css";
import { createVuetify } from "vuetify";

const vuetify = createVuetify({
    locale: { locale: uiLocale.value === "zh-TW" ? "zhHant" : "zhHans", messages: { zhHans, zhHant } },
    theme: {
        defaultTheme: "dark",
    },
});

const app = createApp(App);
app.config.globalProperties.$ui = uiText;
app.config.globalProperties.$uiItems = uiItems;
app.config.globalProperties.$job = jobDisplayName;
watch(uiLocale, value => { vuetify.locale.current.value = value === "zh-TW" ? "zhHant" : "zhHans"; });

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
