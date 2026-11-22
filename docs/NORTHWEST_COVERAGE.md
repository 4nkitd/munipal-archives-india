# Northwest India coverage

The expansion covers municipal corporations in Chandigarh, Delhi, Gujarat, Haryana, Himachal Pradesh, Jammu & Kashmir, Ladakh, Punjab, Rajasthan, Uttarakhand and western Uttar Pradesh. Gujarat is included as the western edge of this project. For Uttar Pradesh, use the Agra, Aligarh, Bareilly, Meerut, Moradabad and Saharanpur divisions, plus other UP districts within the [official NCR boundary](https://ncrpb.nic.in/ncrconstituent.html). This is an archive boundary, not a government definition of northwest India.

## What counts

- **Municipal corporation:** Confirm the legal designation with a state or UT notification, gazette, election authority, or municipal directory. Use [LGD](https://lgdirectory.gov.in/) and [MoHUA City Finance](https://www.cityfinance.in/) to cross-check names and codes. A dated constituting, merging, or abolishing instrument takes precedence over an outdated directory.
- **Historical entity:** Archive documents under the active successor where applicable, and name the authoring body and date explicitly. Do not count predecessor bodies twice. Delhi, for example, has one current MCD, distinct from the New Delhi Municipal Council and Delhi Cantonment Board.
- **Other local bodies:** Councils, committees, nagar panchayats, development authorities and cantonments do not count as corporations. Existing neighboring council archives remain searchable.
- **Name collisions:** Keep distinct bodies separate even if they share a place name. Use `srinagar` for Srinagar, Jammu & Kashmir, and `srinagar-uttarakhand` for Srinagar, Uttarakhand.

## Evidence and gaps

For each corporation, aim for an official profile and records on jurisdiction, wards, citizen services, and finance or notices. Record the issuing authority, publication date or date precision, and source URL for each document. Ward and officer directories are optional when the sources cannot establish individual entries. A statutory ward total and the number of wards mapped by this archive are separate figures.

Only Markdown and JSON belong in `archives/`. Keep source PDFs out of the repository. If a gazette or municipal service is blocked, undated, obsolete, or contradicted by a newer notice, record the limitation. An indexed profile is not evidence that every municipal record or ward can be accessed.

The owner of each collection keeps its evidence notes under `_workspace/northwest/`. Before releasing a region, reconcile its archive directories against the official roster, independently check material facts, and test the built Go index. This document defines the target and acceptance criteria; it does not certify that the target is complete.

## Roster for reconciliation

The following are target archive slugs, not a list of live collections. The working roster is dated 22 November 2026. Check newer gazettes and election notifications before changing it.

| State or UT | Target | Archive slugs |
| --- | ---: | --- |
| [Chandigarh](https://chandigarhdistrict.nic.in/state-election-commission/) | 1 | `chandigarh` |
| [Delhi](https://sec.delhi.gov.in/sec/delimitation-order-2022) | 1 | `delhi` |
| [Gujarat](https://udd.gujarat.gov.in/corporations.php) | 17 | `ahmedabad`, `anand`, `bhavnagar`, `gandhidham`, `gandhinagar`, `jamnagar`, `junagadh`, `mehsana`, `morbi`, `nadiad`, `navsari-vijalpore`, `porbandar`, `rajkot`, `surat`, `surendranagar`, `vadodara`, `vapi` |
| [Haryana](https://ulbharyana.gov.in/Home/StateMunicipalities) | 11 | `ambala`, `faridabad`, `gurugram`, `hisar`, `karnal`, `manesar`, `panchkula`, `panipat`, `rohtak`, `sonipat`, `yamunanagar` |
| [Himachal Pradesh](https://ud.hp.gov.in/page/notification-of-newly-constituted-upgraded-municipalities) | 8 | `baddi`, `dharamshala`, `hamirpur`, `mandi`, `palampur`, `shimla`, `solan`, `una` |
| [Jammu and Kashmir](https://indiacode.gov.in/items/403ec3e0-06ec-41f6-aa7b-b43feba10165) | 2 | `jammu`, `srinagar` |
| [Ladakh](https://ladakh.gov.in/housing-and-urban-development/) | 0 | Leh and Kargil are municipal committees, not corporations. |
| [Punjab](https://e-nigam.punjab.gov.in/) | 14 | `abohar`, `amritsar`, `barnala`, `batala`, `bathinda`, `hoshiarpur`, `jalandhar`, `kapurthala`, `ludhiana`, `moga`, `mohali`, `pathankot`, `patiala`, `phagwara` |
| [Rajasthan](https://lsg.rajasthan.gov.in/) | 10 | `ajmer`, `alwar`, `bharatpur`, `bhilwara`, `bikaner`, `jaipur`, `jodhpur`, `kota`, `pali`, `udaipur` |
| [Uttarakhand](https://udd.uk.gov.in/list-of-ulb/) | 11 | `almora`, `dehradun`, `haldwani-kathgodam`, `haridwar`, `kashipur`, `kotdwar`, `pithoragarh`, `rishikesh`, `roorkee`, `rudrapur`, `srinagar-uttarakhand` |
| [Western Uttar Pradesh](https://e-nagarsewaup.gov.in/ulbappsmain/showUlbList?type=NN) | 10 | `agra`, `aligarh`, `bareilly`, `firozabad`, `ghaziabad`, `mathura-vrindavan`, `meerut`, `moradabad`, `saharanpur`, `shahjahanpur` |

The working target is **85 corporations**, plus the ten pre-existing council and committee archives. The counts are separate so existing council coverage does not hide a missing corporation.

The source snapshot contains **95 bodies, 599 Markdown records, 2,381 mapped ward entries, and 435 officer entries**. Every target corporation has at least one record in addition to its profile. The figures do not imply comprehensive historical records, a complete current ward directory, or confirmed tenure for every listed officer.

## Reading the archive

The city picker and home directory group indexed collections by state or UT. Search across all collections for a document, or select a city before interpreting a ward number. Ward numbers can repeat across cities and can change after delimitation.

The Go corpus tests check the roster, record IDs, ward keys and file types. To confirm the running service uses the expanded corpus, compare `/api/municipalities` with the roster and check that `/api/documents?municipality=<slug>` returns searchable records for each body. The profile's statutory ward count is not the index's mapped-ward count.

Legal and dataset distinctions that affect the tally:

- Punjab's [May 2026 election Gazette](https://sec.punjab.gov.in/media/documents/Gazette_Notification_of_Election.pdf) lists Barnala as a corporation with 50 wards. Older statewide directories may list it as a council.
- Gujarat's [corporation directory](https://udd.gujarat.gov.in/corporations.php) has 17 bodies. The nine newer bodies are not all reflected in older published finance directories.
- Rajasthan's [election programme](https://sec.rajasthan.gov.in/cm/upload/List_merged.pdf) lists Jaipur, Jodhpur and Kota once each. Their earlier split bodies are not separate archive targets.
- Himachal Pradesh's [upgrade notifications](https://ud.hp.gov.in/page/notification-of-newly-constituted-upgraded-municipalities) distinguish the operative 2026 Baddi declaration from the 2024 declaration later quashed by the High Court. Final delimitation does not establish that a corporation has elected members.
