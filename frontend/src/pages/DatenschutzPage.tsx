export default function DatenschutzPage() {
  return (
    <section className="page-section page-section--legal">
      <h1 className="page-title">Datenschutzerklärung</h1>

      <div className="card legal">
        <h2>1. Datenschutz auf einen Blick</h2>
        <p>
          Wir verarbeiten personenbezogene Daten nur, soweit dies zur Bereitstellung des
          Kundenportals der Kfz-Werkstatt erforderlich ist. Diese Erklärung erläutert, welche Daten
          wir zu welchem Zweck verarbeiten und welche Rechte Ihnen zustehen.
        </p>
        <p>
          Verantwortlich für die Datenverarbeitung ist die Kfz-Werkstatt Musterstadt GmbH,
          Werkstattstraße 12, 12345 Musterstadt, Telefon 030 1234567, E-Mail
          kontakt@kfz-werkstatt-musterstadt.example.
        </p>

        <h2>2. Keine externen Ressourcen</h2>
        <p>
          Diese Web-App lädt keine Ressourcen von Drittanbieter-Hosts. Es werden keine
          Schriftarten, Skripte, Analyse-Dienste oder Bilder von externen Servern eingebunden; alle
          Bestandteile der Seite werden über unseren eigenen Server ausgeliefert. Es findet kein
          Tracking und keine Profilbildung statt.
        </p>

        <h2>3. Verarbeitete Daten</h2>
        <p>
          Zur Erstellung einer Terminanfrage verarbeiten wir die von Ihnen angegebenen Daten: Name,
          E-Mail-Adresse, Telefonnummer sowie Fahrzeugdaten (Kennzeichen, Marke, Modell,
          Kilometerstand) und die Beschreibung des Problems. Rechtsgrundlage ist die
          Vertragsanbahnung und Vertragsdurchführung gemäß Art. 6 Abs. 1 lit. b DSGVO.
        </p>
        <p>
          Für die Abfrage des Auftragsstatus und der Rechnung benötigen wir die Auftragsnummer in
          Verbindung mit dem Kennzeichen. Diese Angaben dienen ausschließlich der Zuordnung Ihres
          Auftrags.
        </p>

        <h2>4. Anmeldung im Werkstattbereich</h2>
        <p>
          Der Zugang zum Werkstattbereich ist Mitarbeiterinnen und Mitarbeitern vorbehalten. Die
          Anmeldung erfolgt mit E-Mail-Adresse und Passwort. Passwörter werden ausschließlich als
          Hash gespeichert. Zur Aufrechterhaltung der Sitzung wird ein signiertes Token im Browser
          gespeichert; es enthält keine sensiblen Daten.
        </p>

        <h2>5. Speicherdauer</h2>
        <p>
          Wir speichern Ihre Daten nur so lange, wie es für die Auftragsabwicklung, die
          Rechnungsstellung und die Erfüllung gesetzlicher Aufbewahrungsfristen erforderlich ist.
          Handels- und steuerrechtliche Aufbewahrungsfristen betragen in der Regel sechs bis zehn
          Jahre.
        </p>

        <h2>6. Ihre Rechte</h2>
        <p>
          Sie haben das Recht auf Auskunft (Art. 15 DSGVO), Berichtigung (Art. 16 DSGVO), Löschung
          (Art. 17 DSGVO), Einschränkung der Verarbeitung (Art. 18 DSGVO),
          Datenübertragbarkeit (Art. 20 DSGVO) sowie Widerspruch gegen die Verarbeitung
          (Art. 21 DSGVO). Außerdem haben Sie das Recht, sich bei einer Datenschutz-Aufsichtsbehörde
          zu beschweren.
        </p>

        <h2>7. Verschlüsselung</h2>
        <p>
          Diese Seite nutzt aus Sicherheitsgründen eine TLS-Verschlüsselung. Eine verschlüsselte
          Verbindung erkennen Sie daran, dass die Adresszeile des Browsers von „http://“ auf
          „https://“ wechselt.
        </p>

        <h2>8. Aktualität</h2>
        <p>
          Diese Datenschutzerklärung wird bei Änderungen der Datenverarbeitung angepasst. Es gilt
          die jeweils auf dieser Seite veröffentlichte Fassung.
        </p>
      </div>
    </section>
  )
}
