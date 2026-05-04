function isFulfilled<T>(
  result: PromiseSettledResult<T>,
): result is PromiseFulfilledResult<T> {
  return result.status === "fulfilled";
}

export function fulfilledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
  return isFulfilled(result) ? result.value : fallback;
}
