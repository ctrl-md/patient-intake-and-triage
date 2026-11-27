import { ApiError } from "./api";

/** A message that is safe to show a person. */
export function messageOf(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "Something went wrong. Please try again.";
}
