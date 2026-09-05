// @vitest-environment node
import { describe, it, expect } from "vitest";
import { isLiveHostSessionMessage, classifyHostSessionClose } from "../../utils/host_session.js";

describe("host session navigation", () => {
  it("recognizes an owned Lobby before Question or Score", () => {
    expect(isLiveHostSessionMessage({status:"success",component:"Waiting",event:"send_invitation_code"})).toBe(true);
    expect(isLiveHostSessionMessage({status:"fail",component:"Waiting",data:"session was completed"})).toBe(false);
    expect(isLiveHostSessionMessage({status:"success",component:"Loading"})).toBe(false);
  });
  it("keeps close policy reasons distinct", () => {
    expect(classifyHostSessionClose({code:1008,reason:"orphaned session cannot be resumed"})).toBe("completed");
    expect(classifyHostSessionClose({code:1008,reason:"session already has an admin"})).toBe("duplicate");
    expect(classifyHostSessionClose({code:1008,reason:"unauthorized session owner"})).toBe("unauthorized");
    expect(classifyHostSessionClose({code:1008,reason:"unknown"})).toBe(null);
    expect(classifyHostSessionClose({code:1000,reason:"quiz completed"})).toBe(null);
  });
});
