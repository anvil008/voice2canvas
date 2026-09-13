import { describe, expect, it } from "vitest";
import { calculateBarMax, scaleBarHeight } from "./BarChart";
import { areaPath, scaleXByDate, scaleY, scaleYDomain, smoothPath } from "./chart";

describe("ported chart geometry", () => {
  it("builds stable line and area paths and scales time/value domains", () => {
    const points = [{ x: 0, y: 10 }, { x: 50, y: 0 }, { x: 100, y: 10 }];
    expect(smoothPath(points)).toBe("M 0,10 C 8.3,8.3 33.3,0 50,0 C 66.7,0 91.7,8.3 100,10");
    expect(areaPath(points, 20)).toBe(
      "M 0,20 L 0,10 C 8.3,8.3 33.3,0 50,0 C 66.7,0 91.7,8.3 100,10 L 100,20 Z",
    );
    expect(scaleXByDate(15, 10, 20, 5, 100)).toBe(55);
    expect(scaleY(5, 10, 0, 100)).toBe(50);
    expect(scaleYDomain(0, -10, 10, 10, 100)).toBe(60);
  });

  it("calculates grouped and stacked bar domains and heights", () => {
    const categories = ["A", "B"];
    const series = [
      { name: "One", values: [4, 8] },
      { name: "Two", values: [6, 3] },
    ];
    expect(calculateBarMax(categories, series, false)).toBe(8);
    expect(calculateBarMax(categories, series, true)).toBe(11);
    expect(scaleBarHeight(4, 8, 200)).toBe(100);
    expect(scaleBarHeight(-1, 8, 200)).toBe(0);
  });
});
