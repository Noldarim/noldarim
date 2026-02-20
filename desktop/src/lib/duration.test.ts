import { describe, expect, it } from "vitest";

import { formatDuration } from "./duration";

describe("formatDuration", () => {
  it("returns '0s' for 0ms", () => {
    expect(formatDuration(0)).toBe("0s");
  });

  it("returns '0s' for undefined", () => {
    expect(formatDuration(undefined)).toBe("0s");
  });

  it("returns '0s' for negative values", () => {
    expect(formatDuration(-100)).toBe("0s");
  });

  it("returns milliseconds for values < 1s", () => {
    expect(formatDuration(500)).toBe("500ms");
    expect(formatDuration(1)).toBe("1ms");
    expect(formatDuration(999)).toBe("999ms");
  });

  it("returns seconds for values >= 1s", () => {
    expect(formatDuration(1000)).toBe("1.0s");
    expect(formatDuration(1500)).toBe("1.5s");
    expect(formatDuration(45000)).toBe("45.0s");
  });
});
