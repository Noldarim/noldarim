import { createContext, useContext } from "react";

const ForkFromCommitContext = createContext<((sha: string) => void) | null>(null);

export const ForkFromCommitProvider = ForkFromCommitContext.Provider;

export function useForkFromCommit(): ((sha: string) => void) | null {
  return useContext(ForkFromCommitContext);
}
