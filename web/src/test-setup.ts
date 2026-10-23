import { act } from "@testing-library/react";
import { notifyManager } from "@tanstack/react-query";
import { beforeEach } from "vitest";

// Query notifications are asynchronous external-store updates. Flush them within
// React's test boundary instead of depending on concurrent-render scheduling.
beforeEach(() => {
  notifyManager.setNotifyFunction((callback) => {
    act(() => {
      callback();
    });
  });
});
