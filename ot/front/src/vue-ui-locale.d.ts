import type { uiText, uiItems } from "./uiLocale";
import type { jobDisplayName } from "./gameNameDisplay";
import type { gameUiText } from "./gameTerms";
declare module "vue" {
    interface ComponentCustomProperties {
        $ui: typeof uiText;
        $game: typeof gameUiText;
        $uiItems: typeof uiItems;
        $job: typeof jobDisplayName;
    }
}
export {};
