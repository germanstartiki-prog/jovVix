import constants from "../config/constants.js";

export function isLiveHostSessionMessage(message) {
  return message.status === constants.Success && (
    (message.component === constants.Waiting && message.event === constants.SentInvitaionCode) ||
    message.component === constants.Question || message.component === constants.Score
  );
}

// Close 1008 is shared by several distinct policy rejections.
export function classifyHostSessionClose(event) {
  if (event.code !== 1008) return null;
  switch (event.reason) {
    case "orphaned session cannot be resumed": return "completed";
    case "session already has an admin": return "duplicate";
    case "unauthorized session owner": return "unauthorized";
    default: return null;
  }
}
