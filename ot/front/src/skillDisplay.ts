/**
 * Official CN display names for puppet damage skills. AI variants are genuine
 * skill damage and intentionally share the same user-facing name.
 */
export const PUPPET_DAMAGE_SKILL_NAMES: Readonly<Record<number, string>> = {
    54101: "第2幕: 怒气上涌",
    54102: "第1幕: 偶然的冲突",
    54103: "第4幕: 嫉妒的化身",
    54104: "第6幕: 诱惑陷阱",
    54105: "第7幕: 疯狂地疾走",
    54106: "第9幕: 唤醒的生命",
    54151: "第2幕: 怒气上涌",
    54152: "第1幕: 偶然的冲突",
    54153: "第4幕: 嫉妒的化身",
    54154: "第6幕: 诱惑陷阱",
    54155: "第7幕: 疯狂地疾走",
    54156: "第9幕: 唤醒的生命",
    59167: "重拍坠音",
    59168: "猎踪踏影",
    59169: "终幕绝响",
};

export const SKILL_DISPLAY_NAME_OVERRIDES: Readonly<Record<number, string>> = {
    27012: "托亚灵震爆",
    // The bundled legacy fallback list still carries pre-CN names for these
    // Arcana skills; keep settings/search consistent with the current client.
    59104: "蓄势突击",
    59145: "螺旋爆裂",
};

export function normalizeSkillDisplayName(
    skillId: number,
    resourceName?: string,
): string {
    return SKILL_DISPLAY_NAME_OVERRIDES[skillId]
        || PUPPET_DAMAGE_SKILL_NAMES[skillId]
        || resourceName?.trim()
        || "";
}
