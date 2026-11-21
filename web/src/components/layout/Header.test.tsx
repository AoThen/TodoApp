import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ReactNode } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import Header from './Header';

const makeProps = () => ({
  isOnline: true,
  isSyncing: false,
  onLogout: vi.fn(),
  onOpenAdmin: vi.fn(),
  onOpenPairing: vi.fn(),
  onOpenImport: vi.fn(),
  isAdmin: false,
  bell: undefined as ReactNode,
});

describe('Header', () => {
  beforeEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.theme;
  });

  it('渲染标题与在线状态徽标', () => {
    render(<Header {...makeProps()} />);
    expect(screen.getByText('TodoApp')).toBeInTheDocument();
    expect(screen.getByText('在线')).toBeInTheDocument();
    expect(screen.queryByText('离线')).not.toBeInTheDocument();
  });

  it('离线时显示离线徽标', () => {
    render(<Header {...makeProps()} isOnline={false} />);
    expect(screen.getByText('离线')).toBeInTheDocument();
  });

  it('同步中显示同步指示器', () => {
    render(<Header {...makeProps()} isSyncing />);
    expect(screen.getByText('同步中...')).toBeInTheDocument();
  });

  it('非同步中不显示同步指示器', () => {
    render(<Header {...makeProps()} />);
    expect(screen.queryByText('同步中...')).not.toBeInTheDocument();
  });

  it('isAdmin 为 false 时隐藏管理入口', () => {
    render(<Header {...makeProps()} />);
    expect(screen.queryByText('管理')).not.toBeInTheDocument();
  });

  it('isAdmin 为 true 时显示管理入口并触发 onOpenAdmin', () => {
    const props = makeProps();
    props.isAdmin = true;
    render(<Header {...props} />);
    fireEvent.click(screen.getByText('管理'));
    expect(props.onOpenAdmin).toHaveBeenCalledTimes(1);
  });

  it('登出按钮触发 onLogout', () => {
    const props = makeProps();
    render(<Header {...props} />);
    fireEvent.click(screen.getByText('登出'));
    expect(props.onLogout).toHaveBeenCalledTimes(1);
  });

  it('导入与设备配对按钮触发对应回调', () => {
    const props = makeProps();
    render(<Header {...props} />);
    fireEvent.click(screen.getByText('导入'));
    fireEvent.click(screen.getByText('设备配对'));
    expect(props.onOpenImport).toHaveBeenCalledTimes(1);
    expect(props.onOpenPairing).toHaveBeenCalledTimes(1);
  });

  it('传入的 bell 节点被渲染', () => {
    const props = makeProps();
    props.bell = <div data-testid="bell-node">bell</div>;
    render(<Header {...props} />);
    expect(screen.getByTestId('bell-node')).toBeInTheDocument();
  });

  it('主题切换：写入 dataset 与 localStorage，并切换按钮文案', () => {
    render(<Header {...makeProps()} />);
    const toggle = screen.getByTitle('切换深浅主题');
    expect(toggle).toHaveTextContent('深色');

    fireEvent.click(toggle);
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('theme')).toBe('dark');
    expect(toggle).toHaveTextContent('浅色');

    fireEvent.click(toggle);
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('theme')).toBe('light');
    expect(toggle).toHaveTextContent('深色');
  });

  it('初始为 dark 主题时按钮显示浅色文案', () => {
    document.documentElement.dataset.theme = 'dark';
    render(<Header {...makeProps()} />);
    expect(screen.getByTitle('切换深浅主题')).toHaveTextContent('浅色');
  });
});
