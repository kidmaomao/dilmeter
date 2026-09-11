import { customRef } from 'vue';

export function getMabiNameColor(name: string): string {
    if (!name?.length) {
        return '#808080';
    }

    // https://mabinoger.com/color_sim.htm
    const colCalc = (i: number) => (i * 101) % 97 + 159;

    // if (name.length < 3) {
    //     return '#808080';
    // }

    const ccolor = [0, 0, 0];
    // R = (ASCII Char 1,4,7,10
    // G = (ASCII Char 2,5,8,11
    // B = (ASCII Char 3,6,9,12
    //                          * 101) mod 97) + 159
    for (let i = 0; i < name.length; i++) {
        ccolor[i % 3] += name.charCodeAt(i);
    }
    ccolor[0] = colCalc(ccolor[0]);
    ccolor[1] = colCalc(ccolor[1]);
    ccolor[2] = colCalc(ccolor[2]);

    return '#' + ccolor.map(v => v.toString(16).padStart(2, '0')).join('');
}

export interface IUpdateCallback {
    setUpdateCallback(track: () => void, trigger: () => void): void;
}

export function CustomReactive<T extends IUpdateCallback>(value: T): T {
    const state = customRef<T>((track, trigger) => {
        value.setUpdateCallback(track, trigger);

        return {
            get() {
                track();
                return value;
            },
            set(newValue: T) {
                value = newValue;
                trigger();
            },
        };
    });

    return state.value;
}

export const getSkillIcon = (id: number | string) => {
    const baseUrl =
        "https://cdn.jsdelivr.net/gh/eldisa/mabinogiImage@main/SkillImage/";
    return `${baseUrl}${id}.png`;
};
