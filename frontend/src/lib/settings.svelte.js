import { api } from './api.js';

// Business profile and tax rules. Loaded after sign-in; receipts and the cart read from here.
const blank = () => ({
  loaded: false,
  business_name: 'ChloroPOS',
  legal_name: '',
  address: '',
  phone: '',
  email: '',
  npwp: '',
  receipt_footer: '',
  tax_enabled: false,
  tax_label: 'PPN',
  tax_rate_bps: 1100,
  tax_inclusive: true,
});

export const settings = $state(blank());

// Called whenever the signed-in identity changes, so one shop's profile never shows for another
// account on the same browser, and `loaded` is false until the new account's profile arrives.
export const resetSettings = () => Object.assign(settings, blank());

export async function loadSettings() {
  try {
    Object.assign(settings, await api.get('/settings'));
  } catch {
    // keep defaults; receipts still print with what we have
  }
  settings.loaded = true;
}

export const applySettings = (next) => Object.assign(settings, next);
