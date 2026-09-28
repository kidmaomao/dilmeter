import { MessageType } from '@protobuf-ts/runtime';
import { ResourceData as BaseResourceData } from './resourcedata';

// Prilus /multiclass: ResourceData field 38, MultiClassInfo fields 1/2/6.
// Keep this small display-only extension separate from the legacy generated schema.
// Source: https://prilus.gitlab.io/multiclass (verified 2026-09-28).
export interface MultiClassNames { Id: number; Name: string; SkillBindingIds: number[] }
export const MultiClassNames = new MessageType<MultiClassNames>('prilus.MultiClassNames', [
    { no: 1, name: 'Id', localName: 'Id', kind: 'scalar', T: 13 },
    { no: 2, name: 'Name', localName: 'Name', kind: 'scalar', T: 9 },
    { no: 6, name: 'SkillBindingIds', localName: 'SkillBindingIds', kind: 'scalar', T: 13, repeat: 1 },
]);
export type ResourceData = BaseResourceData & { MultiClassList: MultiClassNames[] };
export const ResourceData = new MessageType<ResourceData>('prilus.ResourceData', [
    ...BaseResourceData.fields,
    { no: 38, name: 'MultiClassList', localName: 'MultiClassList', kind: 'message', T: () => MultiClassNames, repeat: 2 },
]);
