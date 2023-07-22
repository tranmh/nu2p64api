-- -----------------------------------------
-- Mitgliedschaften
-- ----------------------------------------

CREATE OR REPLACE  VIEW vereinsmitglieder_v AS 

SELECT NULL AS UUID
     , mitglied.von AS MitgliedVon
     , mitglied.bis AS MitgliedBis
     , person.bemerkung AS MitgliedBemerkungen
     , CASE WHEN mitglied.bis IS NOT NULL then 0
            WHEN mitglied.stat1 = 1 then 3 -- aktiv
            WHEN mitglied.stat1 = 2 then 2 -- passiv
            ELSE 1
       END  AS MitgliedStatus
     , CASE WHEN mitglied.stat1 = 1 THEN mitglied.von
       ELSE NULL
       END  AS SpielberechtigungVon
     , CASE WHEN mitglied.stat1 = 1 THEN mitglied.bis
       ELSE NULL
       END  AS SpielberechtigungBis
     , mitglied.spielernummer AS SpielberechtigungNr
     , NULL AS VereinUUID
     , org.vkz AS VereinNr
     , org.name AS VereinName
     , NULL AS PersonUUID
     , person.id AS PersonPKZ
     , person.name AS Nachname
     , person.vorname AS Vorname
     , titel.bezeichnung AS Titel
     , person.geburtsdatum AS GebDatum
     , person.geschlecht AS Geschlecht
     , person.nation AS Nation
     , adresse.strasse AS Strasse
     , adresse.plz AS PLZ
     , adresse.ort AS Ort
     , land.bezeichnung AS Land
     , adresse.tel1 AS TelPrivat
     , adresse.tel2 AS TelDienst
     , adresse.tel3 AS TelMobil
     , adresse.fax  AS TelefaxPrivat
     , NULL AS TelefaxDienst
     , adresse.email1 AS Email1
     , adresse.email2 AS Email2
     , NULL AS HTTP
  FROM mitgliedschaft AS mitglied
  JOIN person ON person.id = mitglied.person
  JOIN organisation AS org ON org.id = mitglied.organisation
  LEFT JOIN adresse ON adresse.id = person.adress
  LEFT JOIN land ON land.id = adresse.land
  LEFT JOIN titel ON titel.id = person.titel 
;

-- ------------------------------------------
--  Mitgliedschaften
-- ------------------------------------------

SELECT 
  'UUID'
, 'MitgliedVon'
, 'MitgliedBis'
, 'Bemerkung'
, 'MitgliedStatus'
, 'SpielberechtigungVon'
, 'SpielberechtigungBis'
, 'SpielberechtigungNr'
, 'VereinUUID'
, 'VereinNr'
, 'VereinName'
, 'PersonUUID'
, 'PersonPKZ'
, 'Nachname'
, 'Vorname'
, 'titel'
, 'GebDatum'
, 'geschlecht'
, 'nation'
, 'Strasse'
, 'PLZ'
, 'Ort'
, 'land'
, 'TelPrivat'
, 'TelDienst'
, 'TelMobil'
, 'TelefaxPrivat'
, 'TelefaxDienst'
, 'Email1'
, 'Email2'
, 'HTTP'
, 'MIVIS_MITGLIEDSCHAFT_ID'
, 'MIVIS_PERSON_ID'
, 'MIVIS_ORGANISATION_ID'


UNION

SELECT 
  ifnull(UUID ,'')
, IFNULL(  DATE_FORMAT( MitgliedVon , '%d.%m.%Y' ) ,'')
, IFNULL(  DATE_FORMAT( MitgliedBis , '%d.%m.%Y' ) ,'')
, ifnull(Bemerkung, '')
, MitgliedStatus
, IFNULL(  DATE_FORMAT( SpielberechtigungVon , '%d.%m.%Y' ) ,'')
, IFNULL(  DATE_FORMAT( SpielberechtigungBis , '%d.%m.%Y' ) ,'')
, SpielberechtigungNr
, ifnull(VereinUUID,'')
, VereinNr
, VereinName
, ifnull(PersonUUID,'')
, PersonPKZ
, Nachname
, Vorname
, ifnull(titel,'')
, IFNULL(  DATE_FORMAT( GebDatum , '%d.%m.%Y' ) ,'')
, geschlecht
, IFNULL(  nation  ,'')
, IFNULL(  Strasse  ,'')
, IFNULL(  PLZ  ,'')
, IFNULL(  Ort  ,'')
, IFNULL(  land  ,'')
, IFNULL(  TelPrivat  ,'')
, IFNULL(  TelDienst  ,'')
, IFNULL(  TelMobil  ,'')
, IFNULL(  TelefaxPrivat  ,'')
, IFNULL(  TelefaxDienst  ,'')
, IFNULL(  Email1  ,'')
, IFNULL(  Email2  ,'')
, IFNULL(  HTTP  ,'')
, MIVIS_MITGLIEDSCHAFT_ID
, MIVIS_PERSON_ID
, MIVIS_ORGANISATION_ID


  FROM Vereinsmitglieder_V
  INTO OUTFILE    'export/mitgliedschaften.csv'
  CHARACTER SET 'latin1'
  
  FIELDS TERMINATED BY ';' OPTIONALLY ENCLOSED BY '"'
LINES TERMINATED BY '\r\n' ;

