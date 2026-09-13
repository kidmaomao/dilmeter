export const BOSS_NAME_OVERRIDES: Record<number, string> = {
    7600: "枯木的佩塔克",
    7601: "枯木的佩塔克",
    7602: "布隆塔纳斯",
    7603: "雷内恩的米耶尔",
    7615: "雷内恩的米耶尔：悔恨",
};

/** Catalog metadata is enough to name a boss without loading its event history. */
export function bossDisplayName(
    target: { raceId: number; name: string },
    raceNames: Record<number, string>,
): string {
    const clean = (name: string | undefined) => (name ?? "").replace(/\s+\d+$/, "").trim();
    const resourceName = clean(raceNames[target.raceId]);
    const packetName = clean(target.name);
    // Unnamed monsters often use their entity ID as the packet name.
    return BOSS_NAME_OVERRIDES[target.raceId]
        || (resourceName && !/^\d+$/.test(resourceName) ? resourceName : "")
        || (packetName && !/^\d+$/.test(packetName) ? packetName : "")
        || (target.raceId > 0 ? `首领 ${target.raceId}` : "未识别首领");
}
