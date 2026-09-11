<template>
    <v-btn
        size="x-small"
        variant="text"
        prepend-icon="mdi-account-edit"
        @click="open"
    >
        改名
    </v-btn>

    <v-dialog v-model="dialog" max-width="480px" scrollable>
        <v-card>
            <v-card-title class="d-flex align-center">
                <v-icon class="mr-2">mdi-account-edit</v-icon>
                改變玩家顯示名稱
                <v-spacer />
                <v-btn icon variant="text" @click="dialog = false">
                    <v-icon>mdi-close</v-icon>
                </v-btn>
            </v-card-title>
            <v-divider />

            <v-card-text style="max-height: 400px">
                <div
                    v-if="tempMappings.length === 0"
                    class="text-center text-caption text-disabled py-4"
                >
                    尚無玩家資料
                </div>
                <v-list density="compact" class="pa-0">
                    <v-list-item
                        v-for="m in tempMappings"
                        :key="m.originalName"
                        class="px-0 py-1"
                    >
                        <v-row dense align="center">
                            <v-col cols="5" class="text-caption text-truncate">
                                {{ m.originalName }}
                            </v-col>
                            <v-col cols="1" class="text-center text-disabled">→</v-col>
                            <v-col cols="6">
                                <v-text-field
                                    v-model="m.displayName"
                                    density="compact"
                                    variant="outlined"
                                    hide-details
                                    :placeholder="m.originalName"
                                />
                            </v-col>
                        </v-row>
                    </v-list-item>
                </v-list>
            </v-card-text>

            <v-divider />
            <v-card-actions>
                <v-btn size="small" variant="text" color="error" @click="clearAll">
                    清除全部
                </v-btn>
                <v-spacer />
                <v-btn variant="text" @click="dialog = false">取消</v-btn>
                <v-btn color="primary" variant="flat" @click="apply">套用</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script lang="ts">
import { defineComponent, ref, type PropType } from "vue";
import { idMappings } from "@/store";

export default defineComponent({
    name: "PlayerRenameDialog",
    props: {
        players: {
            type: Array as PropType<{ entityId: string; name: string }[]>,
            default: () => [],
        },
    },
    setup(props) {
        const dialog = ref(false);
        const tempMappings = ref<{ originalName: string; displayName: string }[]>([]);

        const open = () => {
            tempMappings.value = props.players.map((p) => ({
                originalName: p.name,
                displayName: idMappings.value[p.name] || "",
            }));
            dialog.value = true;
        };

        /** 套用：merge 到全域 map（保留其他玩家的改名設定） */
        const apply = () => {
            const next: Record<string, string> = { ...idMappings.value };
            for (const m of tempMappings.value) {
                if (m.displayName.trim()) {
                    next[m.originalName] = m.displayName.trim();
                } else {
                    delete next[m.originalName];
                }
            }
            idMappings.value = next;
            dialog.value = false;
        };

        /** 清除全部：重置 UI 欄位並清空全域 map */
        const clearAll = () => {
            tempMappings.value.forEach((m) => (m.displayName = ""));
            idMappings.value = {};
        };

        return { dialog, tempMappings, open, apply, clearAll };
    },
});
</script>
