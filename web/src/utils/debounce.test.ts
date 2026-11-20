import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { debounce, throttle } from './debounce';

describe('debounce', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('等待期内多次调用只执行最后一次', () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);

    debounced('a');
    vi.advanceTimersByTime(50);
    debounced('b');
    vi.advanceTimersByTime(50);
    debounced('c');
    vi.advanceTimersByTime(100);

    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith('c');
  });

  it('等待期满后才会执行', () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 100);
    debounced('x');
    vi.advanceTimersByTime(99);
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('this 与参数透传', () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 10);
    debounced(1, 2, 3);
    vi.advanceTimersByTime(10);
    expect(fn).toHaveBeenCalledWith(1, 2, 3);
  });
});

describe('throttle', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('窗口期内只执行一次', () => {
    const fn = vi.fn();
    const throttled = throttle(fn, 100);

    throttled('first');
    throttled('ignored1');
    throttled('ignored2');
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith('first');

    vi.advanceTimersByTime(100);
    throttled('second');
    expect(fn).toHaveBeenCalledTimes(2);
    expect(fn).toHaveBeenLastCalledWith('second');
  });
});
