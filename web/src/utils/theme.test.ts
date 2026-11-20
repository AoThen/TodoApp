import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getInitialTheme, applyTheme, saveTheme } from './theme';

function mockMatchMedia(matches: boolean) {
  (window as any).matchMedia = vi.fn().mockReturnValue({ matches });
}

describe('getInitialTheme', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('localStorage 中的 dark 优先于系统偏好', () => {
    localStorage.setItem('theme', 'dark');
    mockMatchMedia(false);
    expect(getInitialTheme()).toBe('dark');
  });

  it('localStorage 中的 light 优先于系统偏好', () => {
    localStorage.setItem('theme', 'light');
    mockMatchMedia(true);
    expect(getInitialTheme()).toBe('light');
  });

  it('无存储且系统偏好深色时跟随系统', () => {
    mockMatchMedia(true);
    expect(getInitialTheme()).toBe('dark');
  });

  it('无存储且系统偏好浅色时使用 light', () => {
    mockMatchMedia(false);
    expect(getInitialTheme()).toBe('light');
  });

  it('存储了非法值时跟随系统', () => {
    localStorage.setItem('theme', 'blue');
    mockMatchMedia(true);
    expect(getInitialTheme()).toBe('dark');
  });
});

describe('applyTheme / saveTheme', () => {
  it('applyTheme 写入 documentElement.dataset', () => {
    applyTheme('dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
    applyTheme('light');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('saveTheme 持久化到 localStorage', () => {
    saveTheme('dark');
    expect(localStorage.getItem('theme')).toBe('dark');
  });
});
