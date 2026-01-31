import React, { useState, useEffect } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { generateKey } from '../utils/crypto';

interface PairingData {
  v: number;
  type: string;
  key: string;
  server: string;
  expires: number;
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

  useEffect(() => {
    const apiUrl = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';
    const baseUrl = apiUrl.replace('/api/v1', '');
    setServerUrl(baseUrl);

    const newKey = generateKey();
    setKey(newKey);

    const expiresIn = 300;
    const pairingData: PairingData = {
      v: 1,
      type: 'todoapp-pairing',
      key: newKey,
      server: baseUrl,
      expires: expiresIn
    };

    setQrData(JSON.stringify(pairingData));
  }, []);

  const handleCopyKey = () => {
    navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleRefreshKey = () => {
    const newKey = generateKey();
    setKey(newKey);
    setCopied(false);

    const expiresIn = 300;
    const pairingData: PairingData = {
      v: 1,
      type: 'todoapp-pairing',
      key: newKey,
      server: serverUrl,
      expires: expiresIn
    };

    setQrData(JSON.stringify(pairingData));
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

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>设备配对</h2>
          <button className="close-btn" onClick={onClose}>&times;</button>
        </div>

        <div className="modal-body">
          <p className="pairing-description">
            使用 TodoApp Android App 扫描此二维码进行设备配对。配对后，所有通信将使用 AES-256-GCM 加密。
          </p>

          {error && <div className="error-message">{error}</div>}

          <div className="qr-container">
            <QRCodeSVG
              id="pairing-qr-code"
              value={qrData}
              size={200}
              level="H"
              includeMargin={true}
            />
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
              >
                {copied ? '已复制!' : '复制'}
              </button>
              <button
                className="btn btn-secondary"
                onClick={handleRefreshKey}
              >
                刷新
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
            >
              下载二维码图片
            </button>
          </div>

          <div className="security-info">
            <h4>安全说明</h4>
            <ul>
              <li>此二维码包含加密密钥，请确保在安全环境下使用</li>
              <li>二维码有效期为 5 分钟，过期后请刷新</li>
              <li>配对成功后，App 将使用此密钥加密所有通信</li>
              <li>密钥存储在 Android Keystore 中，安全性高</li>
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
