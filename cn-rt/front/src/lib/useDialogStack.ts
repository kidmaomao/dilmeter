import { shallowReactive, ref, markRaw, type Component, type Ref } from 'vue'

export interface DialogEntry {
    id: number
    title: Ref<string>
    component: Component
    props: Record<string, any>
}

const dialogs = shallowReactive<DialogEntry[]>([])
let nextId = 0

export function useDialogStack() {
    return {
        dialogs,
        open(component: Component, props: Record<string, any> = {}, title: string = '') {
            const titleRef = ref(title)
            props = { ...props, _dialogTitle: titleRef }
            dialogs.push({ id: nextId++, title: titleRef, component: markRaw(component), props })
        },
        close(id: number) {
            const idx = dialogs.findIndex(d => d.id === id)
            if (idx >= 0) dialogs.splice(idx, 1)
        },
    }
}
