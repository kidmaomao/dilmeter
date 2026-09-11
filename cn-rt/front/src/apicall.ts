import { ref, Ref } from 'vue';
import brotliPromise from 'brotli-dec-wasm';

import { ResourceData, ResourceVersion } from '@/protos/resourcedata';

export const resUrl = ref(`/res/`);

let loadingCount: Ref<number>;

export function resVerCall(path: string, opt?: HttpCallOpt): Promise<ResourceVersion> {
    return httpCall<ResourceVersion>(`${resUrl.value}${path}`, opt);
}

export async function resDataCall(path: string, opt?: HttpCallOpt): Promise<ResourceData> {
    const buf = await httpCallRaw(`${resUrl.value}${path}`, opt);

    try {
        loadingCount.value++;
        const brotli = await brotliPromise;
        const u8 = new Uint8Array(buf);
        
        const dec = brotli.decompress(u8);
        return ResourceData.fromBinary(dec);
    }
    finally {
        loadingCount.value--;
    }
}

export type HttpCallOpt = {
    disableLoading?: boolean;
    reload?: boolean;
};

async function httpCall<T>(url: string, opt?: HttpCallOpt): Promise<T> {
    await setLoadingCount();

    try {
        const buf = await httpCallRaw(url, opt);
        const text = new TextDecoder('utf-8').decode(buf);
        return JSON.parse(text);
    }
    finally {
        if (!opt?.disableLoading) {
            loadingCount.value--;
        }
    }
}

async function httpCallRaw(url: string, opt?: HttpCallOpt): Promise<ArrayBuffer> {
    await setLoadingCount();

    try {
        if (!opt?.disableLoading) {
            loadingCount.value++;
        }

        const r = await fetch(url, {
            cache: opt?.reload ? 'reload' : undefined,
        });
        const buf = await r.arrayBuffer();
        if (r.status != 200) {
            throw new Error(`${r.status} ${new TextDecoder('utf-8').decode(buf)}`);
        }

        return buf;
    }
    finally {
        if (!opt?.disableLoading) {
            loadingCount.value--;
        }
    }
}

// 순환참조 제거용
async function setLoadingCount() {
    if (loadingCount) {
        return;
    }

    const { loadingCount: _loadingCount } = await import('@/store');
    loadingCount = _loadingCount;
}

export async function init(): Promise<void> {
    return;
}