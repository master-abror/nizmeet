/** Satu modul di Web Portal (ADR-0003, butir 16). */
export interface PortalModule {
	/** Sama dengan nama modul di server. */
	id: string;
	label: string;
	/** Jalur halaman utama modul, mis. '/hello'. */
	href: string;
	/** Kunci fitur entitlement: modul tampil di navigasi hanya bila fitur ini aktif. */
	feature: string;
}
