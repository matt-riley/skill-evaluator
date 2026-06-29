import { expect, test, describe } from "vitest";
import { cleanPath, buildNavLinks } from "../src/utils/routing";

describe("routing", () => {
  describe("cleanPath", () => {
    test("given ../../../../README.md returns README", () => {
      expect(cleanPath("../../../../README.md")).toBe("README");
    });

    test("given ../../../../docs/index.md returns index", () => {
      expect(cleanPath("../../../../docs/index.md")).toBe("index");
    });

    test("given ../../../../docs/adr/0001-record.md returns adr/0001-record", () => {
      expect(cleanPath("../../../../docs/adr/0001-record.md")).toBe("adr/0001-record");
    });

    test("given a path ending in .MD (uppercase) returns path without extension", () => {
      expect(cleanPath("../../../../CHANGELOG.MD")).toBe("CHANGELOG");
    });
  });

  describe("buildNavLinks", () => {
    test("orders docs logically, names index as Home, keeps ADRs above Changelog", () => {
      const keys = [
        "../../../../docs/adr/0002-second.md",
        "../../../../docs/setup.md",
        "../../../../docs/adr/0001-record.md",
        "../../../../CHANGELOG.md",
        "../../../../docs/index.md",
      ];
      const navLinks = buildNavLinks(keys);

      expect(navLinks.length).toBe(5);

      expect(navLinks[0].title).toBe("Home");
      expect(navLinks[0].isAdr).toBe(false);

      expect(navLinks[1].title).toBe("Setup");
      expect(navLinks[1].isAdr).toBe(false);

      expect(navLinks[2].title).toBe("0001 Record");
      expect(navLinks[2].isAdr).toBe(true);

      expect(navLinks[3].title).toBe("0002 Second");
      expect(navLinks[3].isAdr).toBe(true);

      expect(navLinks[4].title).toBe("Changelog");
      expect(navLinks[4].isAdr).toBe(false);
      expect(navLinks[4].path).toBe("changelog");
    });
  });
});
