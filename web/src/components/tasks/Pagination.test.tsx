import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import Pagination from './Pagination';

describe('Pagination', () => {
  it('total 为 0 时不渲染', () => {
    const { container } = render(<Pagination page={1} pages={1} total={0} onChange={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('只有一页时不渲染', () => {
    const { container } = render(<Pagination page={1} pages={1} total={5} onChange={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('渲染总数与页码窗口', () => {
    render(<Pagination page={1} pages={10} total={200} onChange={() => {}} />);
    expect(screen.getByText('共 200 条')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '1' })).toHaveClass('active');
    // 窗口 5 页：1-5
    [1, 2, 3, 4, 5].forEach((n) => expect(screen.getByRole('button', { name: String(n) })).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: '6' })).not.toBeInTheDocument();
  });

  it('第一页时上一页禁用，最后一页时下一页禁用', () => {
    const { rerender } = render(<Pagination page={1} pages={10} total={200} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: '上一页' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '下一页' })).toBeEnabled();

    rerender(<Pagination page={10} pages={10} total={200} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: '上一页' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
  });

  it('末页时窗口显示最后 5 页', () => {
    render(<Pagination page={10} pages={10} total={200} onChange={() => {}} />);
    [6, 7, 8, 9, 10].forEach((n) => expect(screen.getByRole('button', { name: String(n) })).toBeInTheDocument());
  });

  it('点击页码与翻页触发 onChange', () => {
    const onChange = vi.fn();
    render(<Pagination page={3} pages={10} total={200} onChange={onChange} />);
    fireEvent.click(screen.getByRole('button', { name: '5' }));
    expect(onChange).toHaveBeenCalledWith(5);
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(onChange).toHaveBeenCalledWith(4);
    fireEvent.click(screen.getByRole('button', { name: '上一页' }));
    expect(onChange).toHaveBeenCalledWith(2);
  });
});
