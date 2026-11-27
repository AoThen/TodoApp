import React, { useState, useEffect } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { apiService } from '../services/api';
import { ensureDeviceKey } from '../services/deviceKey';

interface PairingData {
  v: number;
  type: string;
  key: string;
  server: string;
  /** 过期时间：epoch 秒（Android 端按 optLong 解析） */
  expires: number;
}

interface Device {
  id: number;
  device_type: string;
  device_id: string;
  server_url: string;
  paired_at: string;
  last_seen: string;
  is_active: boolean;
}

interface DevicePairingProps {
  onClose: () => void;
}

const DevicePairing: React.FC<DevicePairingProps> = ({ onClose }) => {
  const [key, setKey] = useState<string>('');
  const [serverUrl, setServerUrl] = useState<string>('');
  const [qrData, setQrData] = useState<string>('');
  const [copied, setCopied] = useState<boolean>(false);
  const [error, setError] = useState<string>('');
  const [success, setSuccess] = useState<string>('');
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [paired, setPaired] = useState<boolean>(false);
  const [showRefreshConfirm, setShowRefreshConfirm] = useState<boolean>(false);

  useEffect(() => {
    const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';
    const baseUrl = apiUrl.replace('/api/v1', '');
    setServerUrl(baseUrl);

    initPairingFlow(baseUrl);

    loadDevices();
  }, []);

  const initPairingFlow = async (baseUrl: string) => {
    try {
      // 第一把钥匙仅用于生成二维码，交由手机扫码消费，网页自身不得占用
      const qrInfo = await apiService.initPairing('web');

      setKey(qrInfo.key);
      setQrData(buildPairingData(qrInfo));

      // 网页自身领取每设备密钥（幂等：已持有则跳过），与二维码使用不同的一次性配对码
      await ensureDeviceKey();

      setPaired(true);
      setSuccess('设备配对成功！');
    } catch (err: any) {
      console.error('初始化配对失败:', err);
      if (err.response?.status === 409) {
        setPaired(true);
      }
    }
  };

  const buildPairingData = (info: {
    key: string;
    server_url: string;
    expires_at: string;
  }): string => {
    const pairingData: PairingData = {
      v: 2,
      type: 'todoapp-pairing',
      key: info.key,
      server: info.server_url,
      expires: Math.floor(new Date(info.expires_at).getTime() / 1000),
    };
    return JSON.stringify(pairingData);
  };

  const loadDevices = async () => {
    try {
      setLoading(true);
      const response = await apiService.getDevices();
      setDevices(response.devices);
    } catch (err) {
      console.error('加载设备列表失败:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleCopyKey = () => {
    navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleRefreshKey = async () => {
    if (!showRefreshConfirm) {
      setShowRefreshConfirm(true);
      setTimeout(() => setShowRefreshConfirm(false), 5000);
      return;
    }

    try {
      setLoading(true);
      
      const pairingInfo = await apiService.initPairing('web');
      
      setKey(pairingInfo.key);

      setQrData(buildPairingData(pairingInfo));

      setSuccess('二维码已刷新');
      setTimeout(() => setSuccess(''), 3000);

      setShowRefreshConfirm(false);
      loadDevices();
    } catch (err: any) {
      setError('密钥生成失败: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleRegenerateKey = async (deviceId: string) => {
    if (!confirm('确定要重新生成该设备的配对密钥吗？旧密钥将失效！')) {
      return;
    }

    try {
      setLoading(true);
      const response = await apiService.regenerateKey(deviceId);
      setSuccess(`密钥已重新生成: ${response.new_key.substr(0, 16)}...`);
      setTimeout(() => setSuccess(''), 5000);
      loadDevices();
    } catch (err: any) {
      setError('密钥生成失败: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeDevice = async (deviceId: string) => {
    if (!confirm('确定要撤销该设备吗？撤销后将无法访问数据！')) {
      return;
    }

    try {
      setLoading(true);
      await apiService.revokeDevice(deviceId);
      setSuccess('设备已撤销');
      setTimeout(() => setSuccess(''), 3000);
      loadDevices();
    } catch (err: any) {
      setError('撤销设备失败: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleDownloadQr = () => {
    const svg = document.getElementById('pairing-qr-code');
    if (svg) {
      const svgData = new XMLSerializer().serializeToString(svg);
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      const img = new Image();
      img.onload = () => {
        canvas.width = img.width;
        canvas.height = img.height;
        ctx?.drawImage(img, 0, 0);
        const pngFile = canvas.toDataURL('image/png');
        const downloadLink = document.createElement('a');
        downloadLink.download = 'todoapp-pairing.png';
        downloadLink.href = pngFile;
        downloadLink.click();
      };
      img.src = 'data:image/svg+xml;base64,' + btoa(svgData);
    }
  };

  const getDeviceTypeName = (type: string): string => {
    const typeMap: Record<string, string> = {
      'web': 'Web',
      'android': 'Android',
      'ios': 'iOS'
    };
    return typeMap[type] || type;
  };

  const getDateStr = (dateStr: string): string => {
    if (!dateStr) return '-';
    try {
      const date = new Date(dateStr);
      return date.toLocaleString('zh-CN');
    } catch {
      return dateStr;
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>设备配对</h2>
          <button className="close-btn" onClick={onClose}>&times;</button>
        </div>

        <div className="modal-body">
          {success && <div className="success-message">{success}</div>}
          {error && <div className="error-message">{error}</div>}

          <p className="pairing-description">
            使用 TodoApp Android App 扫描此二维码进行设备配对。配对后，所有通信将使用 AES-256-GCM 加密。
          </p>

          <div className="qr-container">
            <QRCodeSVG
              id="pairing-qr-code"
              value={qrData}
              size={200}
              level="H"
              includeMargin={true}
            />
            {paired && (
              <div className="paired-badge">
                <span className="paired-icon">✓</span>
                <span>已配对</span>
              </div>
            )}
          </div>

          <div className="key-display">
            <label>配对密钥:</label>
            <div className="key-input-group">
              <input
                type="text"
                value={key}
                readOnly
                className="key-input"
              />
              <button
                className="btn btn-secondary"
                onClick={handleCopyKey}
                disabled={loading}
              >
                {copied ? '已复制!' : '复制'}
              </button>
              <button
                className={`btn ${showRefreshConfirm ? 'btn-warning' : 'btn-secondary'}`}
                onClick={handleRefreshKey}
                disabled={loading}
              >
                {showRefreshConfirm ? '确定刷新？' : '刷新'}
              </button>
            </div>
          </div>

          <div className="server-info">
            <label>服务器地址:</label>
            <span className="server-url">{serverUrl}</span>
          </div>

          <div className="pairing-actions">
            <button
              className="btn btn-secondary"
              onClick={handleDownloadQr}
              disabled={loading}
            >
              下载二维码图片
            </button>
          </div>

          {/* 已配对设备列表 */}
          <div className="paired-devices-section">
            <h3>已配对设备 ({devices.length})</h3>
            {loading && devices.length === 0 ? (
              <div className="loading">加载中...</div>
            ) : devices.length === 0 ? (
              <div className="no-devices">暂无已配对设备</div>
            ) : (
              <div className="devices-list">
                {devices.map(device => (
                  <div key={device.id} className={`device-item ${!device.is_active ? 'revoked' : ''}`}>
                    <div className="device-info">
                      <div className="device-type">
                        {getDeviceTypeName(device.device_type)}
                      </div>
                      <div className="device-id">ID: {device.device_id.substr(0, 20)}...</div>
                      <div className="device-dates">
                        <div>配对时间: {getDateStr(device.paired_at)}</div>
                        <div>最后活跃: {getDateStr(device.last_seen)}</div>
                      </div>
                      {!device.is_active && <div className="device-status revoked">已撤销</div>}
                    </div>
                    {device.is_active && (
                      <div className="device-actions">
                        <button
                          className="btn btn-secondary btn-small"
                          onClick={() => handleRegenerateKey(device.device_id)}
                          disabled={loading}
                          title="重新生成密钥"
                        >
                          重新生成密钥
                        </button>
                        <button
                          className="btn btn-danger btn-small"
                          onClick={() => handleRevokeDevice(device.device_id)}
                          disabled={loading}
                          title="撤销设备"
                        >
                          撤销
                        </button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="security-info">
            <h4>安全说明</h4>
            <ul>
              <li>二维码仅含一次性配对凭据，不含任何长期密钥</li>
              <li>二维码有效期为 5 分钟，过期后请刷新</li>
              <li>通道密钥由各设备经已登录的加密通道单独领取，一台设备一把</li>
              <li>手机端密钥存储在系统安全区（Keystore）中</li>
              <li>网页端密钥仅存于本浏览器，请勿在不可信环境使用，谨防脚本窃取</li>
              <li>撤销设备后该设备密钥立即失效</li>
            </ul>
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-primary" onClick={onClose}>
            完成
          </button>
        </div>
      </div>
    </div>
  );
};

export default DevicePairing;
