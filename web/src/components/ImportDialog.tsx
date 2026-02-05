import React, { useState } from 'react';
import { apiService } from '../services/api';

interface ImportDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onImportSuccess: () => void;
}

interface ImportTask {
  title: string;
  description: string;
  status: string;
  priority: string;
}

interface ImportResponse {
  status: string;
  imported: number;
  skipped: number;
  inserted_ids: number[];
}

const ImportDialog: React.FC<ImportDialogProps> = ({ isOpen, onClose, onImportSuccess }) => {
  const [file, setFile] = useState<File | null>(null);
  const [format, setFormat] = useState<'json' | 'csv'>('json');
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string>('');
  const [previewData, setPreviewData] = useState<ImportTask[] | null>(null);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    e.preventDefault();
    const selectedFile = e.target.files?.[0];
    if (!selectedFile) return;

    setFile(selectedFile);
    setError('');

    if (selectedFile.type.includes('json') || selectedFile.name.endsWith('.json')) {
      setFormat('json');
    } else if (selectedFile.type.includes('csv') || selectedFile.name.endsWith('.csv')) {
      setFormat('csv');
    }

    // 预览文件内容JSON文件
    if (format === 'json') {
      const reader = new FileReader();
      reader.onload = (e: ProgressEvent<FileReader>) => {
        try {
          const result = e.target?.result;
          if (!result) {
            setError("无法读取文件内容");
            return;
          }
          const data = JSON.parse(result as string);
          if (Array.isArray(data)) {
            setPreviewData(data);
          } else {
            setError("文件格式必须是任务数组");
          }
        } catch (err) {
          setError("不是有效的JSON文件");
        }
      };
      reader.readAsText(selectedFile);
    }
  };

  const handleUpload = async () => {
    if (!file) {
      setError('请选择文件');
      return;
    }

    setIsUploading(true);
    setError('');

    const formData = new FormData();
    formData.append('file', file);
    formData.append('format', format);

    try {
      const response = await apiService.importTasks(formData);
      
      alert(`导入成功！\n已导入: ${response.imported}\n跳过: ${response.skipped}`);
      onImportSuccess();
      onClose();
    } catch (error: any) {
      console.error('导入失败:', error);
      const errorMsg = error.response?.data?.error || error.message || '导入失败';
      setError(errorMsg);
    } finally {
      setIsUploading(false);
    }
  };

  const handleDownloadTemplate = () => {
    const templateJSON = [
      {
        title: "示例任务1",
        description: "这是一个示例任务",
        status: "todo",
        priority: "high",
        dueAt: new Date(Date.now() + 24*60*60*1000).toISOString(),
      },
      {
        title: "任务详情",
        description: "需要详细描述",
        status: "in_progress",
        priority: "medium",
        dueAt: new Date(Date.now() + 7*24*60*60*1000).toISOString(),
      },
      {
        title: "完成报告",
        description: "需要本周完成",
        status: "todo",
        priority: "low",
      },
    ];

    const blob = new Blob([JSON.stringify(templateJSON, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'tasks-import-template.json';
    a.click();
    URL.revokeObjectURL(url);
  };

  if (!isOpen) return null;

  return (
    <div className="modal-overlay">
      <div className="modal-content">
        <div className="modal-header">
          <h2>导入任务</h2>
          <button className="close-btn" onClick={onClose}>&times;</button>
        </div>

        <div className="modal-body">
          {error && <div className="error-message">{error}</div>}

          <div className="import-options">
            <label>
              <input 
                type="radio" 
                value="json"
                checked={format === 'json'}
                onChange={(e) => setFormat(e.target.value as 'json' | 'csv')}
              />
              JSON 格式
            </label>
            <label>
              <input 
                type="radio" 
                value="csv"
                checked={format === 'csv'}
                onChange={(e) => setFormat(e.target.value as 'json' | 'csv')}
              />
              CSV 格式
            </label>
          </div>

          <div className="file-input-group">
            <input 
              type="file" 
              accept={format === 'json' ? '.json' : '.csv'}
              onChange={handleFileChange}
              disabled={isUploading}
            />
          </div>

          <div className="download-template">
            <button 
              type="button"
              className="btn btn-secondary"
              disabled={isUploading}
              onClick={handleDownloadTemplate}
            >
              下载导入模板
            </button>
          </div>

          {previewData && format === 'json' && (
            <div className="import-preview">
              <h4>预览数据 ({previewData.length}条任务)</h4>
              <div className="preview-list">
                {previewData.slice(0, 5).map((task, index) => (
                  <div key={index} className="preview-item">
                    <span>{task.title}</span>
                    <small>{task.status}/{task.priority}</small>
                  </div>
                ))}
                {previewData.length > 5 && <p>...</p>}
              </div>
            </div>
          )}

          <div className="import-upload">
            <button 
              className="btn btn-primary" 
              onClick={handleUpload} 
              disabled={isUploading || !file}
            >
              {isUploading ? '导入中...' : '开始导入'}
            </button>
          </div>

          <div className="import-help">
            <h4>导入说明</h4>
            <ul>
              <li><strong>支持格式:</strong> JSON (.json) 和 CSV (.csv)</li>
              <li><strong>CSV字段顺序:</strong> title, description, status, priority, due_at</li>
              <li><strong>默认值:</strong> 空值字段将自动填充默认值</li>
              <li><strong>跳过规则:</strong> 空标题的任务将被跳过</li>
              <li><strong>模板下载:</strong> 可下载示例JSON模板</li>
            </ul>
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>取消</button>
          <button className="btn btn-primary" onClick={handleUpload} disabled={isUploading || !file}>
            {isUploading ? '导入中...' : '开始导入'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default ImportDialog;
