import { getFeatures } from '../lib/entitlements';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = () => ({ features: getFeatures() });
