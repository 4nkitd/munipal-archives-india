# India coverage

Nagar Archive's national expansion covers legally active municipal corporations in India's 28 states and eight union territories. The original ten councils and committees remain searchable but do not count toward corporation coverage. A state or UT without an active corporation is recorded as having none; it does not get a placeholder archive.

The [national roster](india-roster.tsv) dated 22 November 2026 lists **279 corporations**. The [retained councils and committees](retained-councils.tsv) bring the release target to **289 bodies**. The roster lists targets; only the release check establishes what the index actually serves.

## Roster evidence

- The Ministry of Panchayati Raj's [Local Government Directory](https://lgdirectory.gov.in/) dashboard displayed 276 municipal corporations in 5,050 urban local bodies on 22 November 2026. The [19 November LGD-derived export](https://github.com/ramSeraph/opendata/releases/download/lgd-latest/urban_local_bodies.19Nov2026.csv.7z) is a third-party mirror. Its uncompressed SHA-256 is `3352957d5371dfba1f07ea4f5f2af94f6ce27148af2f998dcdcf1a5d5d1b639f`. Its 276 type-4 rows are a starting list, not the legal tally. The official full download and state summary require a CAPTCHA.
- The Ministry of Housing and Urban Affairs' [City Finance ULB feed](https://www.cityfinance.in/api/v1/ulb) and [state feed](https://www.cityfinance.in/api/v1/state) provide independent names and codes. The 22 November snapshot returned 5,039 ULB rows, of which 242 were both active and published and labeled `Municipal Corporation`. This feed is a finance directory; some classifications predate mergers or upgrades.
- Prefer a dated government gazette or constituting, merging, or abolishing order. Where possible, corroborate it with a state department, election commission, or municipal portal. Distinguish the date of an order, its publication, and the date it took effect. Count the legal body once, regardless of aliases or archived predecessor records.

### Examples that change the count

- [Unified Delhi](https://sec.delhi.gov.in/sec/about-sec-delhi) counts once, not as the three predecessor corporations still in the LGD extract. New Delhi Municipal Council remains separate.
- [Tamil Nadu Gazette 257](https://www.stationeryprinting.tn.gov.in/extraordinary/2024/257_Ex_II_2_2024.pdf) was published on 10 August 2024 and took effect on 12 August. LGD has Karaikudi and Pudukkottai as corporations but still shows Namakkal and Tiruvannamalai as municipalities.
- Jharkhand's [state directory](https://udhd.jharkhand.gov.in/ULB/Hariharganj/Hariharganj.aspx) and [2026 Palamu election gazette index](https://palamu.nic.in/municipal-election-2020-3/) classify Hariharganj as a Nagar Panchayat. Its LGD type-4 entry is not legal evidence that it is a corporation. Jamshedpur's industrial township is separate from Mango Municipal Corporation.
- Maharashtra's [final 2025–26 election statement](https://mahasec.maharashtra.gov.in/Site/Upload/PDF/Municipal%20Corporations%20final%20party%20wise%20position.pdf) includes all 29 corporations. City Finance still classifies Jalna as a council.
- The Assam Gazette established both [Silchar](https://dpns.assam.gov.in/document-details/no-737-e-228921-508-dated-28-11-2022) and [Dibrugarh](https://dpns.assam.gov.in/document-details/no-116-e-242733-125-dated-06-03-24). They count alongside Guwahati although the LGD snapshot lists only Guwahati as a corporation.
- Karnataka's [Greater Bengaluru Authority](https://gba.karnataka.gov.in/) coordinates five city corporations constituted under Gazette No. 522 on 2 September 2025. BBMP was their predecessor. GBA is not a sixth corporation.
- [Telangana's acts and gazettes](https://law.telangana.gov.in/actsNew) establish 12 corporations. Three of LGD's 11 rows are absorbed predecessors, and four newer corporations are still typed as municipalities there. The later CURE Act preserves Greater Hyderabad, Cyberabad and Malkajgiri as separate corporations.
- Punjab's Barnala took effect as a corporation on 18 November 2025. Both national directories still classify it as a council; the [Punjab election gazette](https://sec.punjab.gov.in/media/documents/Gazette_Notification_of_Election.pdf) names it as a corporation.

The seven jurisdictions without a statutory municipal corporation are Andaman and Nicobar Islands, Dadra and Nagar Haveli and Daman and Diu, Ladakh, Lakshadweep, Meghalaya, Nagaland, and Puducherry. They can have other kinds of local authorities. For example, Sri Vijaya Puram is a municipal council, and Shillong is a municipal board.

## What qualifies as covered

A corporation needs a profile and a substantive record grounded in an official source. A directory row or a link alone is not coverage. Add separate ward, service, and finance or notice records where the evidence supports them. Leave unverified publication dates and structured rows unset. A published ward count does not establish ward boundaries or officeholders' current tenure.

Before deployment, reconcile every state and UT against the accepted roster, compare the exact archive and indexed slug sets, verify each municipality has searchable records, and exercise citations and hybrid retrieval against the packaged binary. An unresolved legal classification or a missing corporation blocks the national release.
