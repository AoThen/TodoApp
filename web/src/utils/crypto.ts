const ENCRYPTION_KEY = process.env.REACT_APP_ENCRYPTION_KEY || '';

interface EncryptedData {
  iv: string;
  ciphertext: string;
  tag: string;
}

async function deriveKey(keyMaterial: ArrayBuffer): Promise<CryptoKey> {
  const key = await crypto.subtle.importKey(
    'raw',
    keyMaterial,
    { name: 'AES-GCM' },
    false,
    ['encrypt', 'decrypt']
  );
  return key;
}

async function getKeyFromHex(hexKey: string): Promise<CryptoKey> {
  const keyBytes = new Uint8Array(hexKey.match(/.{1,2}/g)!.map(byte => parseInt(byte, 16)));
  return deriveKey(keyBytes.buffer);
}

export async function encrypt(plaintext: string, keyHex?: string): Promise<string> {
  const key = keyHex || ENCRYPTION_KEY;
  if (!key) {
    throw new Error('Encryption key not configured');
  }

  const cryptoKey = await getKeyFromHex(key);
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const encoder = new TextEncoder();
  const data = encoder.encode(plaintext);

  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    cryptoKey,
    data
  );

  const combined = new Uint8Array(iv.length + ciphertext.byteLength);
  combined.set(iv);
  combined.set(new Uint8Array(ciphertext), iv.length);

  return btoa(String.fromCharCode(...combined));
}

export async function decrypt(encryptedBase64: string, keyHex?: string): Promise<string> {
  const key = keyHex || ENCRYPTION_KEY;
  if (!key) {
    throw new Error('Encryption key not configured');
  }

  const cryptoKey = await getKeyFromHex(key);
  const combined = new Uint8Array(
    atob(encryptedBase64).split('').map(c => c.charCodeAt(0))
  );

  const iv = combined.slice(0, 12);
  const ciphertext = combined.slice(12);

  const decrypted = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv },
    cryptoKey,
    ciphertext
  );

  const decoder = new TextDecoder();
  return decoder.decode(decrypted);
}

export function generateKey(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
}

export function isEncryptionConfigured(): boolean {
  return !!ENCRYPTION_KEY;
}
