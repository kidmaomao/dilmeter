import { ComputedRef, Ref } from 'vue';

import { MabiDB } from '@/mabidb';
import { ActorManager } from '@/eventActor';
import { DamageCollectorManager } from '@/actionCollector';

declare module '@vue/runtime-core' {
    function inject(key: 'isLoading'): ComputedRef<boolean>;
    function inject(key: 'region'): Ref<string>;
    function inject(key: 'lang'): Ref<string>;
    function inject(key: 'regionList'): Ref<string[]>;
    function inject(key: 'db'): ComputedRef<MabiDB>;
    function inject(key: 'raceNameMap'): Ref<Record<number, string>>;
    function inject(key: 'skillNameMap'): Ref<Record<number, string>>;
    function inject(key: 'condNameMap'): Ref<Record<number, string>>;
    function inject(key: 'itemNameMap'): Ref<Record<number, string>>;
    function inject(key: 'appEvent'): Ref<EventTarget>;
    function inject(key: 'actorManager'): Ref<ActorManager>;
    function inject(key: 'dcManager'): Ref<DamageCollectorManager>;
    function inject(key: 'timeRangeMin'): Ref<number | null>;
    function inject(key: 'timeRangeMax'): Ref<number | null>;
    function inject(key: 'hasTimeRange'): ComputedRef<boolean>;
}