-- -----------------------------------------
-- Hilfsviews
-- -----------------------------------------

CREATE OR REPLACE VIEW kontakt_verband_v AS 

SELECT org.id AS organisation_id
     , a.strasse AS strasse
     , a.plz AS plz
     , a.ort AS ort
     , land.bezeichnung AS land
     , a.tel1 AS tel_privat
     , a.tel2 AS tel_dienst
     , a.tel3 AS tel_mobil
     , a.fax AS fax_privat
     , NULL AS fax_dienst
     , a.email1 AS email1
     , a.email2 AS email2
     , hp.wert AS http
  FROM organisation AS org
  LEFT JOIN adressen AS brief ON brief.organisation = org.id AND brief.typ = 1      
  LEFT JOIN adresse AS a ON a.id = org.adress
  LEFT JOIN land ON land.id = a.land
  LEFT JOIN adr AS hp ON hp.id_adressen = brief.id AND hp.id_art = 15
 WHERE (brief.istperson = 0 OR brief.id IS NULL)
   AND org.organisationsart > 20

UNION

SELECT org.id AS organisation_id
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
  FROM organisation AS org
  LEFT JOIN adressen AS brief ON brief.organisation = org.id AND brief.typ = 1      
  LEFT JOIN funktion ON funktion.organisation = brief.organisation
                    AND funktion.funktion = brief.funktion
	              AND funktion.bis IS NULL
  LEFT JOIN person pers ON pers.id = funktion.person
  LEFT JOIN adresse pers_adr ON pers_adr.id = pers.adress
  LEFT JOIN land ON land.id = pers_adr.land
  LEFT JOIN adr AS hp ON hp.id_adressen = brief.id AND hp.id_art = 15
 WHERE brief.istperson = 1
   AND org.organisationsart > 20
;

-- -----------------------------------------
-- Verbandsstammdaten
-- ----------------------------------------

CREATE OR REPLACE VIEW verbandsstammdaten_v 

SELECT NULL AS UUID
     , org.name AS VerbandName
     , org.kurzname AS VerbandKurzname
     , org.id AS VerbandNummer
     , NULL AS RegionUUID
     , CASE WHEN org.organisationsart = 70 then 1
            WHEN org.organisationsart = 60 THEN 2
            WHEN org.organisationsart = 50 THEN 3
            WHEN org.organisationsart = 40 AND org.verband = 'C' THEN 3
            WHEN org.organisationsart = 40 THEN 4
            WHEN org.organisationsart = 30 THEN 4
            ELSE 5
       END AS Hierarchie
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
  FROM organisation org
  LEFT JOIN kontakt_verband_v kontakt ON kontakt.organisation_id = org.id
 WHERE org.organisationsart > 20  
;

-- ------------------------------------------
--  Verbandsstammdaten
-- ------------------------------------------

SELECT 
   'UUID'
 , 'VerbandName'
 , 'VerbandKurzname'
 , 'VerbandNummer'
 , 'RegionUUID'
 , 'Hierarchie'
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

UNION

SELECT 
  ifnull(UUID, '')
 , ifnull(VerbandName, '')
 , ifnull(VerbandKurzname, '')
 , ifnull(VerbandNummer, '')
 , ifnull(RegionUUID, '')
 , ifnull(Hierarchie, '')
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

  FROM verbandsstammdaten_v
  INTO OUTFILE    'export/Verbandsstammdaten.csv'
  CHARACTER SET 'latin1'
  
  FIELDS TERMINATED BY ';' OPTIONALLY ENCLOSED BY '"'
LINES TERMINATED BY '\r\n' ;