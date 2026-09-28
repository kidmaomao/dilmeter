import { ref, Ref } from 'vue';
import brotliPromise from 'brotli-dec-wasm';

import { ResourceVersion } from '@/protos/resourcedata';
import { ResourceData } from '@/protos/resourceNames';

function getDefaultResourceUrl() {
    if (__IS_STANDALONE__) return 'https://mabires.pril.cc/';

    // The desktop build carries a CN snapshot so opening the application and
    // resolving skill/boss names never depends on the network. The dev server
    // serves this same public directory, including on non-default ports.
    return '/local-res/';
}

export const resUrl = ref(getDefaultResourceUrl());

export function resourceUrl(path: string): string {
    // TW uses the same official resource service as Prilus. Desktop requests
    // use the existing proxy; CN retains the bundled/external-pack route.
    if (/^(resourcedata|resourceversion)\/tw\/tw_/.test(path)) {
        return `${__IS_STANDALONE__ ? 'https://mabires.pril.cc/' : '/res/'}${path}`;
    }
    return `${resUrl.value}${path}`;
}

let loadingCount: Ref<number>;

export function resVerCall(path: string, opt?: HttpCallOpt): Promise<ResourceVersion> {
    return httpCall<ResourceVersion>(resourceUrl(path), opt);
}

export async function resDataCall(path: string, opt?: HttpCallOpt): Promise<ResourceData> {
    const buf = await httpCallRaw(resourceUrl(path), opt);

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
    const buf = await httpCallRaw(url, opt);
    const text = new TextDecoder('utf-8').decode(buf);
    return JSON.parse(text);
}

async function httpCallRaw(url: string, opt?: HttpCallOpt): Promise<ArrayBuffer> {
    await setLoadingCount();

    try {
        if (!opt?.disableLoading) {
            loadingCount.value++;
        }

        const r = await fetch(url, {
            cache: opt?.reload ? 'reload' : undefined,
            signal: AbortSignal.timeout(30000),
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
