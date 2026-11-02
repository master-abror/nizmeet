import { helloModule } from './hello';
import type { PortalModule } from './types';

/**
 * Satu-satunya tempat modul portal didaftarkan (ADR-0003, butir 16).
 * Menambah modul = folder route group baru + satu entri di sini.
 */
export const portalModules: PortalModule[] = [helloModule];

/** Modul yang fiturnya aktif, sesuai urutan registrasi. */
export function visibleModules(features: readonly string[]): PortalModule[] {
	return portalModules.filter((m) => features.includes(m.feature));
}
