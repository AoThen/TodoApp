import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ImportDialog from './ImportDialog';
import { apiService } from '../services/api';

vi.mock('../services/api', () => ({
  apiService: { importTasks: vi.fn() },
}));

const mockedImport = vi.mocked(apiService.importTasks);

function pickFile(input: HTMLElement, file: File) {
  Object.defineProperty(input, 'files', { value: [file], configurable: true });
  fireEvent.change(input);
}

describe('ImportDialog', () => {
  beforeEach(() => {
    mockedImport.mockReset().mockResolvedValue({ status: 'imported', imported: 2, skipped: 0, inserted_ids: [1, 2] } as never);
    vi.stubGlobal('alert', vi.fn());
  });

  it('isOpen=false 时不渲染', () => {
    const { container } = render(<ImportDialog isOpen={false} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it('未选择文件时上传按钮禁用（无法触发导入）', () => {
    render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    const uploadButtons = screen.getAllByRole('button', { name: '开始导入' });
    expect(uploadButtons.every(b => (b as HTMLButtonElement).disabled)).toBe(true);
    expect(mockedImport).not.toHaveBeenCalled();
  });

  it('选择 JSON 文件后展示预览并按 JSON 导入', async () => {
    const { container } = render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;

    const json = JSON.stringify([{ title: '任务1', status: 'todo', priority: 'high' }]);
    const file = new File([json], 'tasks.json', { type: 'application/json' });
    pickFile(input, file);

    // FileReader 异步读取预览
    await waitFor(() => expect(screen.getByText(/预览数据/)).toBeInTheDocument());
    expect(screen.getByText('任务1')).toBeInTheDocument();

    const uploadButtons = screen.getAllByRole('button', { name: '开始导入' });
    fireEvent.click(uploadButtons[0]);

    await waitFor(() => expect(mockedImport).toHaveBeenCalledTimes(1));
    const formData = mockedImport.mock.calls[0][0] as FormData;
    expect(formData.get('format')).toBe('json');
    expect(formData.get('file')).toBe(file);
  });

  it('导入成功后回调并关闭', async () => {
    const onImportSuccess = vi.fn();
    const onClose = vi.fn();
    const { container } = render(<ImportDialog isOpen={true} onClose={onClose} onImportSuccess={onImportSuccess} />);

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    pickFile(input, new File(['[{"title":"t"}]'], 't.json', { type: 'application/json' }));
    await waitFor(() => expect(screen.getByText(/预览数据/)).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: '开始导入' })[0]);
    await waitFor(() => {
      expect(onImportSuccess).toHaveBeenCalledTimes(1);
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it('导入失败时展示后端错误信息', async () => {
    mockedImport.mockRejectedValue({ response: { data: { error: '文件格式不支持' } } } as never);
    const { container } = render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    pickFile(input, new File(['[]'], 't.json', { type: 'application/json' }));
    await waitFor(() => expect(screen.getByText(/预览数据/)).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: '开始导入' })[0]);
    await waitFor(() => expect(screen.getByText('文件格式不支持')).toBeInTheDocument());
  });

  it('切换 CSV 格式', () => {
    render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    const csvRadio = screen.getByRole('radio', { name: /CSV/ }) as HTMLInputElement;
    fireEvent.click(csvRadio);
    expect(csvRadio.checked).toBe(true);
  });

  it('选择非法 JSON 文件时展示解析错误', async () => {
    const { container } = render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    pickFile(input, new File(['not-valid-json'], 'bad.json', { type: 'application/json' }));
    await waitFor(() => expect(screen.getByText('不是有效的JSON文件')).toBeInTheDocument());
  });
});

describe('ImportDialog 模板下载', () => {
  it('点击下载模板触发模板文件下载，模板使用 due_at 字段', async () => {
    let capturedBlob: Blob | null = null;
    const createSpy = vi.fn((blob: Blob) => { capturedBlob = blob; return 'blob:template-url'; });
    const revokeSpy = vi.fn();
    vi.stubGlobal('URL', { ...URL, createObjectURL: createSpy, revokeObjectURL: revokeSpy });
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    render(<ImportDialog isOpen={true} onClose={vi.fn()} onImportSuccess={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: '下载导入模板' }));

    expect(createSpy).toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    expect(revokeSpy).toHaveBeenCalledWith('blob:template-url');

    const text = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = () => reject(reader.error);
      reader.readAsText(capturedBlob!);
    });
    expect(text).toContain('"due_at"');
    expect(text).not.toContain('dueAt');

    clickSpy.mockRestore();
    vi.unstubAllGlobals();
  });
});
