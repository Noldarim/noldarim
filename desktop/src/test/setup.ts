import "@testing-library/jest-dom";

// ReactFlow's internal zustand store triggers state updates in mount effects
// that cannot be wrapped in act() from test code. Suppress the resulting
// console.error warnings to keep test output clean.
const originalError = console.error;
console.error = (...args: unknown[]) => {
  if (typeof args[0] === "string" && args[0].includes("was not wrapped in act")) {
    return;
  }
  originalError.call(console, ...args);
};

class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

Object.defineProperty(window, "ResizeObserver", {
  value: ResizeObserver
});

Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
  configurable: true,
  value: 900
});

Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
  configurable: true,
  value: 1400
});
