import type { uiText, uiItems } from "./uiLocale";
import type { jobDisplayName } from "./gameNameDisplay";
declare module "vue" {
    interface ComponentCustomProperties {
        $ui: typeof uiText;
        $uiItems: typeof uiItems;
        $job: typeof jobDisplayName;
    }
}
export {};
