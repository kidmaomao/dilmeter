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
    24103: "逆龙袭",
    24201: "连续技：升龙裂破",
    24301: "连续技：飞身踢",
    27000: "多尔卡精通",
    27012: "托亚灵震爆",
    59046: "魔法穿刺",
    59047: "烬火引燃",
    59085: "牺牲之惩戒",
    59088: "高贵的誓约",
    // The bundled legacy fallback list still carries pre-CN names for these
    // Arcana skills; keep settings/search consistent with the current client.
    59104: "蓄势突击",
    59145: "螺旋爆裂",
    // Confirmed against the controlled reworked puppeteer capture.
    59165: "间奏斩",
    59185: "疾风突刺",
    59186: "愤怒践踏",
    59187: "烈拳三击",
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
