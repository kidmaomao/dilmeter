package main

// The preview owns its lifetime; live target updates cannot erase it. The
// original deadline stays fixed even while the report keeps sending frames.
// Caller holds skillOverlayState's lock.
func mergeTargetHealthPreviewLocked(message nativeSkillOverlayMessage, nowMs int64) nativeSkillOverlayMessage {
	incoming := message.TargetHealth
	previous := skillOverlayState.healthPreview
	if previous != nil && previous.PreviewEndsAt <= nowMs {
		skillOverlayState.healthPreview = nil
		previous = nil
	}
	if incoming != nil && incoming.PreviewEndsAt > nowMs && incoming.MaximumHealth > 0 && incoming.CurrentHealth >= 0 {
		if previous == nil || incoming.Generation >= previous.Generation {
			preview := *incoming
			preview.PreviewEndsAt = min(preview.PreviewEndsAt, nowMs+8000)
			if previous != nil && incoming.Generation == previous.Generation {
				preview.PreviewEndsAt = min(preview.PreviewEndsAt, previous.PreviewEndsAt)
			}
			skillOverlayState.healthPreview = &preview
		}
	}
	if preview := skillOverlayState.healthPreview; preview != nil {
		if preview.PreviewEndsAt > nowMs {
			copy := *preview
			message.TargetHealth = &copy
		} else {
			skillOverlayState.healthPreview = nil
		}
	}
	if message.TargetHealth != nil && message.TargetHealth.PreviewEndsAt > 0 && message.TargetHealth.PreviewEndsAt <= nowMs {
		message.TargetHealth = nil
	}
	return message
}
