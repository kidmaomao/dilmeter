import type { BuffAlertRule } from "@/buffAlert";
import type { BossMechanicAlertSettings } from "@/bossMechanicAlert";
import type { DebuffAlertSettings } from "@/debuffAlert";
import type { AimReminderSettings, SkillCooldownRule } from "@/skillCooldown";
import type { EffectTimerSettings } from "@/effectTimer";

export interface ReminderProfileSnapshot {
    buffRules: Record<number, BuffAlertRule>;
    debuffSettings: DebuffAlertSettings;
    aimReminder: AimReminderSettings;
    skillRules: Record<number, SkillCooldownRule>;
    bossMechanicSettings: BossMechanicAlertSettings;
    effectTimerSettings: EffectTimerSettings;
    playerBuffIds: number[];
    playerBuffFavoriteIds: number[];
    playerBuffUsesDefaults: boolean;
}

export interface ReminderProfile extends ReminderProfileSnapshot {
    id: string;
    name: string;
    updatedAt: number;
}

export interface ReminderProfileStore {
    activeProfileId: string;
    profiles: ReminderProfile[];
}

export const REMINDER_PROFILE_STORAGE_KEY = "dilmeter-cn-reminder-profiles-v1";

export function loadReminderProfileStore(fallbackSnapshot: ReminderProfileSnapshot): ReminderProfileStore {
    const fallback = makeInitialStore(fallbackSnapshot);
    try {
        const raw = localStorage.getItem(REMINDER_PROFILE_STORAGE_KEY);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw) as Partial<ReminderProfileStore>;
        if (!Array.isArray(parsed.profiles) || !parsed.profiles.length) return fallback;
        const profiles = parsed.profiles
            .map((profile, index) => sanitizeProfile(profile, fallbackSnapshot, index))
            .filter((profile): profile is ReminderProfile => Boolean(profile));
        if (!profiles.length) return fallback;
        const activeProfileId = profiles.some((profile) => profile.id === parsed.activeProfileId)
            ? String(parsed.activeProfileId)
            : profiles[0].id;
        return { activeProfileId, profiles };
    } catch {
        return fallback;
    }
}

export function saveReminderProfileStore(store: ReminderProfileStore): void {
    const profiles = store.profiles.slice(0, 30).map((profile, index) =>
        sanitizeProfile(profile, emptySnapshot(), index),
    ).filter((profile): profile is ReminderProfile => Boolean(profile));
    if (!profiles.length) return;
    const activeProfileId = profiles.some((profile) => profile.id === store.activeProfileId)
        ? store.activeProfileId
        : profiles[0].id;
    localStorage.setItem(REMINDER_PROFILE_STORAGE_KEY, JSON.stringify({ activeProfileId, profiles }));
}

export function cloneReminderSnapshot(snapshot: ReminderProfileSnapshot): ReminderProfileSnapshot {
    return {
        buffRules: cloneRecord(snapshot.buffRules),
        debuffSettings: cloneDebuffSettings(snapshot.debuffSettings),
        aimReminder: cloneAimReminderSettings(snapshot.aimReminder),
        skillRules: cloneRecord(snapshot.skillRules),
        bossMechanicSettings: cloneBossMechanicSettings(snapshot.bossMechanicSettings),
        effectTimerSettings: cloneEffectTimerSettings(snapshot.effectTimerSettings),
        playerBuffIds: sanitizeIds(snapshot.playerBuffIds),
        playerBuffFavoriteIds: sanitizeIds(snapshot.playerBuffFavoriteIds),
        playerBuffUsesDefaults: Boolean(snapshot.playerBuffUsesDefaults),
    };
}

export function makeReminderProfile(
    name: string,
    snapshot: ReminderProfileSnapshot,
    id = `profile-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
): ReminderProfile {
    return {
        id,
        name: sanitizeName(name) || "新方案",
        updatedAt: Date.now(),
        ...cloneReminderSnapshot(snapshot),
    };
}

function makeInitialStore(snapshot: ReminderProfileSnapshot): ReminderProfileStore {
    const profile = makeReminderProfile("默认方案", snapshot, "default");
    return { activeProfileId: profile.id, profiles: [profile] };
}

function sanitizeProfile(
    value: unknown,
    fallback: ReminderProfileSnapshot,
    index: number,
): ReminderProfile | null {
    if (!value || typeof value !== "object") return null;
    const profile = value as Partial<ReminderProfile>;
    const id = typeof profile.id === "string" && profile.id.trim()
        ? profile.id.trim().slice(0, 80)
        : `profile-${index + 1}`;
    const name = sanitizeName(profile.name) || `方案 ${index + 1}`;
    const snapshot = cloneReminderSnapshot({
        buffRules: profile.buffRules && typeof profile.buffRules === "object" ? profile.buffRules : fallback.buffRules,
        debuffSettings: profile.debuffSettings && typeof profile.debuffSettings === "object"
            ? profile.debuffSettings
            : fallback.debuffSettings,
        aimReminder: profile.aimReminder && typeof profile.aimReminder === "object"
            ? profile.aimReminder
            : fallback.aimReminder,
        skillRules: profile.skillRules && typeof profile.skillRules === "object" ? profile.skillRules : fallback.skillRules,
        bossMechanicSettings: profile.bossMechanicSettings && typeof profile.bossMechanicSettings === "object"
            ? profile.bossMechanicSettings
            : fallback.bossMechanicSettings,
        effectTimerSettings: profile.effectTimerSettings && typeof profile.effectTimerSettings === "object"
            ? profile.effectTimerSettings
            : fallback.effectTimerSettings,
        playerBuffIds: Array.isArray(profile.playerBuffIds) ? profile.playerBuffIds : fallback.playerBuffIds,
        playerBuffFavoriteIds: Array.isArray(profile.playerBuffFavoriteIds)
            ? profile.playerBuffFavoriteIds
            : fallback.playerBuffFavoriteIds,
        playerBuffUsesDefaults: typeof profile.playerBuffUsesDefaults === "boolean"
            ? profile.playerBuffUsesDefaults
            : fallback.playerBuffUsesDefaults,
    });
    return {
        id,
        name,
        updatedAt: Number.isFinite(Number(profile.updatedAt)) ? Number(profile.updatedAt) : Date.now(),
        ...snapshot,
    };
}

function cloneRecord<T>(value: Record<number, T>): Record<number, T> {
    try {
        return JSON.parse(JSON.stringify(value ?? {})) as Record<number, T>;
    } catch {
        return {};
    }
}

function cloneDebuffSettings(value: DebuffAlertSettings): DebuffAlertSettings {
    try {
        return JSON.parse(JSON.stringify(value)) as DebuffAlertSettings;
    } catch {
        return emptyDebuffSettings();
    }
}

function cloneBossMechanicSettings(value: BossMechanicAlertSettings): BossMechanicAlertSettings {
    try {
        return JSON.parse(JSON.stringify(value)) as BossMechanicAlertSettings;
    } catch {
        return emptyBossMechanicSettings();
    }
}

function cloneEffectTimerSettings(value: EffectTimerSettings): EffectTimerSettings {
    try {
        return JSON.parse(JSON.stringify(value ?? { rules: {} })) as EffectTimerSettings;
    } catch {
        return { rules: {} };
    }
}

function cloneAimReminderSettings(value: AimReminderSettings): AimReminderSettings {
    const fallback = defaultAimReminderSettings();
    return {
        enabled: value?.enabled !== false,
        alwaysVisible: Boolean(value?.alwaysVisible),
        weaponRange: clampNumber(value?.weaponRange, 100, 10000, fallback.weaponRange),
        rangeIdentificationLevel: Math.round(clampNumber(
            value?.rangeIdentificationLevel,
            0,
            20,
            fallback.rangeIdentificationLevel,
        )),
        calibrationPercent: clampNumber(value?.calibrationPercent, 20, 40, fallback.calibrationPercent),
        ergSpeedPercent: clampNumber(value?.ergSpeedPercent, 100, 1000, fallback.ergSpeedPercent),
        fineTuneSeconds: clampNumber(value?.fineTuneSeconds, -10, 10, fallback.fineTuneSeconds),
        scalePercent: clampNumber(value?.scalePercent, 50, 200, fallback.scalePercent),
        x: clampNumber(value?.x, -32000, 32000, fallback.x),
        y: clampNumber(value?.y, -32000, 32000, fallback.y),
    };
}

function sanitizeIds(value: number[]): number[] {
    return [...new Set((value ?? []).map(Number).filter((id) => Number.isInteger(id) && id >= 0))]
        .sort((a, b) => a - b);
}

function sanitizeName(value: unknown): string {
    if (typeof value !== "string") return "";
    return value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, 32);
}

function emptySnapshot(): ReminderProfileSnapshot {
    return {
        buffRules: {},
        debuffSettings: emptyDebuffSettings(),
        aimReminder: defaultAimReminderSettings(),
        skillRules: {},
        bossMechanicSettings: emptyBossMechanicSettings(),
        effectTimerSettings: { rules: {} },
        playerBuffIds: [],
        playerBuffFavoriteIds: [],
        playerBuffUsesDefaults: false,
    };
}

function clampNumber(value: unknown, min: number, max: number, fallback: number): number {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) return fallback;
    return Math.min(max, Math.max(min, numeric));
}

function defaultAimReminderSettings(): AimReminderSettings {
    return {
        enabled: true,
        alwaysVisible: false,
        weaponRange: 2200,
        rangeIdentificationLevel: 0,
        calibrationPercent: 40,
        ergSpeedPercent: 200,
        fineTuneSeconds: 0,
        scalePercent: 100,
        x: 600,
        y: 180,
    };
}

function emptyBossMechanicSettings(): BossMechanicAlertSettings {
    return {
        volume: 80,
        x: 850,
        y: 280,
        scalePercent: 100,
        mielShardHealthPhases: { normal80: false, normal60: true, normal40: false, regret80: false },
        mielShardHealthBarX: 750,
        mielShardHealthBarY: 280,
        mielShardHealthBarScalePercent: 100,
        mielShardHealthBarOpacityPercent: 100,
        rules: {},
    };
}

function emptyDebuffSettings(): DebuffAlertSettings {
    return {
        overlayEnabled: true,
        firstRoundGraceSeconds: 10,
        iconSize: 30,
        volume: 80,
        forcedBossRaceIds: [],
        soundMode: "electronic",
        customSoundId: "",
        customSoundName: "",
        rules: {},
    };
}
