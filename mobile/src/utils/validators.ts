const ETHIOPIAN_PHONE_REGEX = /^(?:\+251|0)?(9|7)\d{8}$/;

export function isValidEthiopianPhone(phone: string): boolean {
  const cleaned = phone.replace(/[\s-]/g, '');
  return ETHIOPIAN_PHONE_REGEX.test(cleaned);
}

export function normalizeEthiopianPhone(phone: string): string {
  const cleaned = phone.replace(/[\s\-]/g, '');

  if (cleaned.startsWith('+251')) {
    return cleaned;
  }

  if (cleaned.startsWith('251')) {
    return `+${cleaned}`;
  }

  if (cleaned.startsWith('0')) {
    return `+251${cleaned.slice(1)}`;
  }

  return `+251${cleaned}`;
}

export function isValidPIN(pin: string): boolean {
  if (!/^\d{4,6}$/.test(pin)) {
    return false;
  }

  const allSame = /^(\d)\1+$/.test(pin);
  if (allSame) {
    return false;
  }

  const sequential = ['012345', '123456', '543210', '654321', '987654', '456789'];
  if (sequential.some((seq) => pin.includes(seq.slice(0, pin.length)))) {
    return false;
  }

  return true;
}

export function isValidTrackingNumber(number: string): boolean {
  const cleaned = number.replace(/[^A-Za-z0-9]/g, '');
  return /^[A-Z]{2,3}\d{6,12}$/i.test(cleaned) || /^[A-Z]{2,3}-\d{3}-\d{3,6}$/i.test(number);
}