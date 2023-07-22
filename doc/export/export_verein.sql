-- Views anlegen

-- -----------------------------------------
-- Hilfsviews
-- -----------------------------------------

CREATE OR REPLACE  VIEW kontakt_person_v AS 

SELECT org.id AS organisation_id
     , CONCAT(pers.vorname, ' ', pers.name) AS name
     , pers_adr.strasse AS strasse
     , pers_adr.plz AS plz
     , pers_adr.ort AS ort
     , land.bezeichnung AS land
     , pers_adr.tel1 AS tel_privat
     , pers_adr.tel2 AS tel_dienst
     , pers_adr.tel3 AS tel_mobil
     , pers_adr.fax AS fax_privat
     , NULL AS fax_dienst
     , pers_adr.email1 AS email1
     , pers_adr.email2 AS email2
     , hp.wert AS http
     , NULL AS laengengrad
     , NULL AS breitengrad
  FROM organisation AS org
  JOIN adressen AS kontakt ON kontakt.organisation = org.id AND kontakt.typ = 3
  JOIN person AS pers ON pers.id = kontakt.person 
  LEFT JOIN adresse pers_adr ON pers_adr.id = pers.adress  
  LEFT JOIN land ON land.id = pers_adr.land  
  LEFT JOIN adressen brief ON brief.organisation = org.id AND brief.typ = 1
  LEFT JOIN adr AS hp ON hp.id_adressen = brief.id AND hp.id_art = 15
;

CREATE OR REPLACE  VIEW rechnung_person_v AS 

SELECT org.id AS organisation_id
     , CONCAT(nam.wert, ' ', zus.wert) AS name
     , str.wert AS strasse
     , plz.wert AS plz
     , ort.wert AS ort
     , land.wert AS land
     , tel1.wert AS tel_privat
     , tel2.wert AS tel_dienst
     , tel3.wert AS tel_mobil
     , fax.wert AS fax_privat
     , NULL AS fax_dienst
     , mail1.wert AS email1
     , mail2.wert AS email2
     , hp.wert AS http
     , NULL AS laengengrad
     , NULL AS breitengrad
  FROM organisation AS org
  JOIN adressen AS rech ON rech.organisation = org.id AND rech.typ = 2      
  LEFT JOIN adr nam ON nam.id_adressen = rech.id AND nam.id_art = 1
  LEFT JOIN adr zus ON zus.id_adressen = rech.id AND zus.id_art = 12
  LEFT JOIN adr str ON str.id_adressen = rech.id AND str.id_art = 2
  LEFT JOIN adr plz ON plz.id_adressen = rech.id AND plz.id_art = 3
  LEFT JOIN adr ort ON ort.id_adressen = rech.id AND ort.id_art = 4
  LEFT JOIN adr land ON land.id_adressen = rech.id AND land.id_art = 5
  LEFT JOIN adr tel1 ON tel1.id_adressen = rech.id AND tel1.id_art = 6
  LEFT JOIN adr tel2 ON tel2.id_adressen = rech.id AND tel2.id_art = 7
  LEFT JOIN adr tel3 ON tel3.id_adressen = rech.id AND tel3.id_art = 8
  LEFT JOIN adr fax ON fax.id_adressen = rech.id AND fax.id_art = 9
  LEFT JOIN adr mail1 ON mail1.id_adressen = rech.id AND mail1.id_art = 10
  LEFT JOIN adr mail2 ON mail2.id_adressen = rech.id AND mail2.id_art = 11
  LEFT JOIN adressen brief ON brief.organisation = org.id AND brief.typ = 1
  LEFT JOIN adr AS hp ON hp.id_adressen = brief.id AND hp.id_art = 15
 WHERE rech.istperson = 0

UNION

SELECT org.id AS organisation_id
     , CONCAT(pers.vorname, ' ', pers.name) AS name
     , pers_adr.strasse as strasse
     , pers_adr.plz AS plz
     , pers_adr.ort AS ort
     , land.bezeichnung AS land
     , pers_adr.tel1 AS tel_privat
     , pers_adr.tel2 AS tel_dienst
     , pers_adr.tel3 AS tel_mobil
     , pers_adr.fax AS fax_privat
     , NULL AS fax_dienst
     , pers_adr.email1 AS email1
     , pers_adr.email2 AS email2
     , hp.wert AS http
     , NULL AS laengengrad
     , NULL AS breitengrad
  FROM organisation AS org
  JOIN adressen AS rech ON rech.organisation = org.id AND rech.typ = 2      
  JOIN funktion ON funktion.organisation = rech.organisation
               AND funktion.funktion = rech.funktion
		   AND funktion.bis IS NULL
  JOIN person pers ON pers.id = funktion.person
  LEFT JOIN adresse pers_adr ON pers_adr.id = pers.adress
  LEFT JOIN land ON land.id = pers_adr.land
  LEFT JOIN adressen brief ON brief.organisation = org.id AND brief.typ = 1
  LEFT JOIN adr AS hp ON hp.id_adressen = brief.id AND hp.id_art = 15
 WHERE rech.istperson = 1
;

-- -----------------------------------------
-- Vereinsstammdaten
-- ----------------------------------------

CREATE OR REPLACE  VIEW vereinsstammdaten_v AS 

SELECT NULL as UUID
     , verband.kurzname AS VerbandKurzname
     , NULL AS VerbandUUID
     , org.vkz AS VereinNr
     , org.name AS VereinName
     , org.sportbundnr AS VereinLSBNummer
     , CASE WHEN mit_org.bis IS null
	        THEN 1
			ELSE 0
		END AS VereinStatus 
	 , NULL AS RegionUUID
     , NULL AS RegionName
     , YEAR(org.grundungsdatum) AS GruendungJahr
     , NULL AS BankName
     , NULL AS BankBIC
     , NULL AS BankKontoInhaber
     , NULL AS BankIBAN
     , NULL AS ZahlungLastschrift
     , kontakt.name AS Name_Kontakt
     , kontakt.strasse AS Strasse_Kontakt
     , kontakt.plz AS PLZ_Kontakt
     , kontakt.ort AS Ort_Kontakt
     , kontakt.land AS Land_Kontakt
     , kontakt.tel_privat AS TelPrivat_Kontakt
     , kontakt.tel_dienst AS TelDienst_Kontakt
     , kontakt.tel_mobil AS TelMobil_Kontakt
     , kontakt.fax_privat AS TelefaxPrivat_Kontakt
     , kontakt.fax_dienst AS TelefaxDienst_Kontakt
     , kontakt.email1 AS Email1_Kontakt
     , kontakt.email2 AS Email2_Kontakt
     , kontakt.http AS HTTP_Kontakt
     , kontakt.laengengrad AS Laengengrad_Kontakt
     , kontakt.breitengrad AS Breitengrad_Kontakt
     , rechnung.name AS Name_Rechnung
     , rechnung.strasse AS Strasse_Rechnung
     , rechnung.plz AS PLZ_Rechnung
     , rechnung.ort AS Ort_Rechnung
     , rechnung.land AS Land_Rechnung
     , rechnung.tel_privat AS TelPrivat_Rechnung
     , rechnung.tel_dienst AS TelDienst_Rechnung
     , rechnung.tel_mobil AS TelMobil_Rechnung
     , rechnung.fax_privat AS TelefaxPrivat_Rechnung
     , rechnung.fax_dienst AS TelefaxDienst_Rechnung
     , rechnung.email1 AS Email1_Rechnung
     , rechnung.email2 AS Email2_Rechnung
     , rechnung.http AS HTTP_Rechnung
  FROM organisation AS org
  LEFT JOIN mitgliedschaftOrganisation AS mit_org ON mit_org.organisation = org.id
  LEFT JOIN kontakt_person_v AS kontakt ON kontakt.organisation_id = org.id
  LEFT JOIN rechnung_person_v AS rechnung ON rechnung.organisation_id = org.id
  LEFT JOIN organisation verband ON verband.verband = org.verband AND verband.organisationsart = 60
 WHERE org.organisationsart = 20
;

-- ------------------------------------------
--  Vereinsstammdaten
-- ------------------------------------------

SELECT 
   'UUID'
 , 'VerbandKurzname'
 , 'VerbandUUID'
 , 'VereinNr'
 , 'VereinName'
 , 'VereinLSBNummer'
 , 'VereinStatus'
 , 'RegionUUID'
 , 'RegionName'
 , 'GruendungJahr'
 , 'BankName'
 , 'BankBIC'
 , 'BankKontoInhaber'
 , 'BankIBAN'
 , 'ZahlungLastschrift'
 , 'Name_Kontakt'
 , 'Strasse_Kontakt'
 , 'PLZ_Kontakt'
 , 'Ort_Kontakt'
 , 'Land_Kontakt'
 , 'TelPrivat_Kontakt'
 , 'TelDienst_Kontakt'
 , 'TelMobil_Kontakt'
 , 'TelefaxPrivat_Kontakt'
 , 'TelefaxDienst_Kontakt'
 , 'Email1_Kontakt'
 , 'Email2_Kontakt'
 , 'HTTP_Kontakt'
 , 'Laengengrad_Kontakt'
 , 'Breitengrad_Kontakt'
 , 'Name_Rechnung'
 , 'Strasse_Rechnung'
 , 'PLZ_Rechnung'
 , 'Ort_Rechnung'
 , 'Land_Rechnung'
 , 'TelPrivat_Rechnung'
 , 'TelDienst_Rechnung'
 , 'TelMobil_Rechnung'
 , 'TelefaxPrivat_Rechnung'
 , 'TelefaxDienst_Rechnung'
 , 'Email1_Rechnung'
 , 'Email2_Rechnung'
 , 'HTTP_Rechnung'

UNION

SELECT 
  ifnull(UUID, '')
 , ifnull(VerbandKurzname, '')
 , ifnull(VerbandUUID, '')
 , ifnull(VereinNr, '')
 , ifnull(VereinName, '')
 , ifnull(VereinLSBNummer, '')
 , ifnull(VereinStatus, '')
 , ifnull(RegionUUID, '')
 , ifnull(RegionName, '')
 , ifnull(GruendungJahr, '')
 , ifnull(BankName, '')
 , ifnull(BankBIC, '')
 , ifnull(BankKontoInhaber, '')
 , ifnull(BankIBAN, '')
 , ifnull(ZahlungLastschrift, '')
 , ifnull(Name_Kontakt, '')
 , ifnull(Strasse_Kontakt, '')
 , ifnull(PLZ_Kontakt, '')
 , ifnull(Ort_Kontakt, '')
 , ifnull(Land_Kontakt, '')
 , ifnull(TelPrivat_Kontakt, '')
 , ifnull(TelDienst_Kontakt, '')
 , ifnull(TelMobil_Kontakt, '')
 , ifnull(TelefaxPrivat_Kontakt, '')
 , ifnull(TelefaxDienst_Kontakt, '')
 , ifnull(Email1_Kontakt, '')
 , ifnull(Email2_Kontakt, '')
 , ifnull(HTTP_Kontakt, '')
 , ifnull(Laengengrad_Kontakt, '')
 , ifnull(Breitengrad_Kontakt, '')
 , ifnull(Name_Rechnung, '')
 , ifnull(Strasse_Rechnung, '')
 , ifnull(PLZ_Rechnung, '')
 , ifnull(Ort_Rechnung, '')
 , ifnull(Land_Rechnung, '')
 , ifnull(TelPrivat_Rechnung, '')
 , ifnull(TelDienst_Rechnung, '')
 , ifnull(TelMobil_Rechnung, '')
 , ifnull(TelefaxPrivat_Rechnung, '')
 , ifnull(TelefaxDienst_Rechnung, '')
 , ifnull(Email1_Rechnung, '')
 , ifnull(Email2_Rechnung, '')
 , ifnull(HTTP_Rechnung, '')

  FROM vereinsstammdaten_v
  INTO OUTFILE    'export/Vereinsstammdaten.csv'
  CHARACTER SET 'latin1'
  
  FIELDS TERMINATED BY ';' OPTIONALLY ENCLOSED BY '"'
LINES TERMINATED BY '\r\n' ;
