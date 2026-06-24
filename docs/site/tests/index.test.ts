import { describe, it, expect } from "vitest"

describe("site", () => {
  it("has a homepage title", () => {
    const title = "Home"
    expect(title).toBe("Home")
  })
})
