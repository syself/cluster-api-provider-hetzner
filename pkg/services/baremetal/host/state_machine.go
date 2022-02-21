package host

import (
	"fmt"

	infrav1 "github.com/syself/cluster-api-provider-hetzner/api/v1beta1"
)

// hostStateMachine is a finite state machine that manages transitions between
// the states of a BareMetalHost.
type hostStateMachine struct {
	host       *infrav1.HetznerBareMetalHost
	reconciler *Service
	nextState  infrav1.ProvisioningState
}

func newHostStateMachine(host *infrav1.HetznerBareMetalHost, reconciler *Service) *hostStateMachine {
	currentState := host.Spec.Status.ProvisioningState
	r := hostStateMachine{
		host:       host,
		reconciler: reconciler,
		nextState:  currentState, // Remain in current state by default
	}
	return &r
}

type stateHandler func(*reconcileInfo) actionResult

func (hsm *hostStateMachine) handlers() map[infrav1.ProvisioningState]stateHandler {
	return map[infrav1.ProvisioningState]stateHandler{
		infrav1.StateNone:           hsm.handleNone,
		infrav1.StateAvailable:      hsm.handleAvailable,
		infrav1.StatePreparing:      hsm.handleNone,
		infrav1.StatePrepared:       hsm.handleNone,
		infrav1.StateProvisioning:   hsm.handleNone,
		infrav1.StateProvisioned:    hsm.handleNone,
		infrav1.StateDeprovisioning: hsm.handleNone,
		infrav1.StateDeleting:       hsm.handleNone,
	}
}

func (hsm *hostStateMachine) ReconcileState(info *reconcileInfo) (actionRes actionResult) {
	initialState := hsm.host.Spec.Status.ProvisioningState

	if stateHandler, found := hsm.handlers()[initialState]; found {
		return stateHandler(info)
	}

	info.log.Info("No handler found for state", "state", initialState)
	return actionError{fmt.Errorf("No handler found for state \"%s\"", initialState)}
}

// handleNone checks whether server exists in Hetzner Robot API, then checks whether
// the SSH key exists in Robot API, and finally decides whether server is already in
// rescue mode and sets the next state accordingly.
func (hsm *hostStateMachine) handleNone(info *reconcileInfo) actionResult {
	actResult := hsm.reconciler.actionNone(info)
	if _, ok := actResult.(actionComplete); ok {
		hsm.nextState = infrav1.StateRegistering
	}
	return actResult
}

func (hsm *hostStateMachine) handleRegistering(info *reconcileInfo) actionResult {
	server, actResult := hsm.reconciler.verifyReboot(info)
	if _, ok := actResult.(actionComplete); ok {
		// Check whether server needs to be set in rescue state
		if server.Rescue {
			hsm.nextState = infrav1.StateRegistering
		} else {
			hsm.nextState = infrav1.StateRescueSystem
		}
	}
	return actResult
}

func (hsm *hostStateMachine) handleAvailable(info *reconcileInfo) actionResult {
	// StateAvailable boots the machine in the rescue system. (api hetzner rescue + reboot)
	// Gathers informations about the machine. (Storage, NIC, CPU, RAM) -> spec.status

	return actionComplete{}
}

// 1. (None) nachschauen in hetzner api ob node existiert. Wenn nicht dann error state mit "not found"
// 2. (Available) Wir warten bis die machine ausgewählt wurde, ein image gesetzt wurde. Wenn ja, gehe zu Schritt 3
// 3. (Preparing) Rescue System (API) und reboot (API)
// 3b: Check ob man mit Rescue SSH auf die Maschine kommt. Wenn dies für x min nicht klappt, gehe zu Schritt 3. Sonst Schritt 3c
// 3c: SSH befehle ausführen um an Hardware Details zu dem node zu gelangen.
//		Infos werden in spec.status gespeichert. Root Device Hints und Festplatten vergleichen und eine Festplatte auswählen.
//		Der DeviceName der Festplatte wird behalten und beim erstellen der autosetup Datei für die OS festplatte genommen.
//	  Die autosetup datei übertragen mit scp oder auf dem node ausführen mit cat
// 3d. Installimage ausführen und anschließend rebooten. Mit reboot befehl (ssh). 2 sekunden warten.
// 4. (prepared) Check ob reboot durchgeführt wurde.
//		1. Wenn nicht, dann noch einmal ausgeführen (per SSH). 5 sek warten.
// 		Wenn es schon einmal per SSH versucht wurde und immer noch nicht funktioniert, dann restart über Hetzner API.
// 		Wenn es nach 20 Minuten nicht geklappt hat, in einen Error state.
// 5. Check ob reboot fertig ist. SSH Key installierte Maschine. Bei error warten und normal probieren.  5min restart über Hetzner API. Wenn nach weiteren 20 min immer noch nichts passiert ist, kommt er in einen endgültigen error state
// 4. (Provisioning) Mit SSH auf die installierte Maschine (SSH User von der normalen Maschine). Wir installieren cloud-init.
//		Erstellen ein paar dateien für cloud-init und übertragen die user_data datei. Anschließend reboot.
//		Das hier ist äquivalent zu server erstellen mit user_data. Hiernach müssen wir nichts mehr tun die maschine sollte dann im cluster erscheinen.
// 5. (Provisioned)

// Von rescue zu OS.
// 1. Schritt 4. Prüfen ob wir mit Rescue SSH Keys drauf kommen.
// 	Wenn wir immer noch ins rescue system kommen.
// 	1. Reboot befehl
// 	2. warten 5s
// 	Wenn wir immer noch ins rescue system kommen.
// 		1. Reboot über api
// 		2. warten 5min
// 	Wenn wir immer noch ins rescue system kommen.
// 		1. Error state
// 2. Schritt 5. Prüfen ob wir mit OS SSH Keys drauf kommen.
// 		Wenn wir nach x min nicht drauf kommen.
// 			1. reboot über api (GEHE ZURÜCK AUF 132)

// Schritt 4:

// Schritt 4a: Reboot über SSH
// Has no error message yet:

// Können wir uns mit OS SSH Keys verbinden?
// 	Ja:
// 		Gehe zu Schritt 5
// 	Nein:
// 		Has no error message / Has ErrorMessage SSHRebootInitializationFailed:
// 			// Überprüfe, ob SSH reboot funktioniert hat
// 			Ja:
// 				Können wir uns mit rescue SSH keys verbinden?
// 					Ja:
// 						Has error message SSHRebootInitializationFailed:
// 							Ja:
// 								Lösche error message SSHRebootInitializationFailed
// 								Reboot per API  (error msg: SSHRebootInitializationCompletelyFailed)
// 							Nein:
// 								Reboot per SSH (error msg: SSHRebootInitializationFailed)
// 					Nein:
// 						Können wir uns mit OS SSH Keys verbinden?
// 							Ja:
// 								Gehe zu Schritt 5
// 							Nein:
// 								error msg: ConnectionToHostSystemFailed
// 								reconcile again
// 	Has ErrorMessage RebootInitializationCompletelyFailed:
// 		Ja:
// 			erstelle error message APIRebootFailed
// 			lösche error message RebootInitializationCompletelyFailed

// 	Has ErrorMessage APIRebootFailed:
// 		Ja:
// 			Has error message for longer than 20 min:
// 				Ja:
// 					Gehe zu Error state
// 				Nein:
// 					Reconcile again

// Has error message RebootInitializationFailed:
// 	Ja:
// 		Können wir uns mit rescue SSH keys verbinden?
// 			Ja:
// 				Gehe zu Schritt 4b: Reboot über API
// 			Nein:
// 				Lösche error message RebootInitializationFailed
// 				Können wir uns mit OS SSH Keys verbinden?
// 					Ja:
// 						Gehe zu Schritt 5
// 					Nein:
// 						Has error message ConnectionToHostSystemFailed for longer than 5 min:
// 							Ja:
// 								Gehe zu Schritt 4b: Reboot über API
// 							Nein:
// 								Wenn noch nicht vorhanden: error message:  ConnectionToHostSystemFailed
// 								reconcile again

// Has error message ConnectionToHostSystemFailed:
// 	Können wir uns mit OS SSH Keys verbinden?
// 		Ja:
// 			Gehe zu Schritt 5
// 		Nein:
// 			Has error message ConnectionToHostSystemFailed for longer than 5 min:
// 				Ja:
// 					Gehe zu Schritt 4b: Reboot über API
// 				Nein:
// 					reconcile again

// Schritt 4b:

// Von OS zu Rescue.

/*
Neue Notes:
1. Prüfe secrets: sshkey und robotCreds (Immer bzw. nur wenn Resource Version höher geworden ist)
2. StateNone: Checke ob Server existiert in Robot API. Prüfe, ob wir im Rescue System sind. Wenn nein, -> Wechsele zu StateRescueSystem
Wenn ja, dann gehe in StateRegistering.
3. StateRescueSystem: Wir fordern rescue System per API (boot/id/rescue) an. Dann ein API reboot/reset mit hw. Mache dann die gleiche Logik
wie unten bzgl. Testen ob SSH Verbindung aufgebaut werden kann und ob der Server power on ist. Wenn es klappt dann gehe in StateRegistering
3. StateRegistering: Hole Hardware details, überprüfe ob RootDeviceHints gesetzt sind. Wenn nicht, dann gehe in Error (ohne noch einmal reconcile)
Wenn ja, dann überprüfe ob root device hints zu einer Festplatte führen. Wenn ja, gehe in available. Wenn nein, gehe in error.
4. StateAvailable: Wenn xxx passiert, dann prüfe ob Server in Rescue ist. Wenn ja, gehe zu StateImageInstalling. Wenn nein, dann
gehe zu StateImageInstallingRescue
5a. StateImageInstallingRescue: Schalte Rescue System ein und mache SSH reboot (wenn es klappt) oder  API reboot plus Handling wenn
das Ganze nicht funktioniert
5. StateImageInstalling:
	1. Prüfe ob in Rescue System.
	1. Storage devices holen (lsblk -b -P -o "NAME,LABEL,FSTYPE,TYPE,HCTL,MODEL,VENDOR,SERIAL,SIZE,WWN,ROTA)
	2. Wählen Festplatte von type "disk" mit der richtigen WWN aus root device hints aus und merken uns Namen
	3. Wir haben ein script in einer config map, wo wir ein go template raus machen sollten, um die Variablen os_device, hostname und image
	einfügen zu können. Die COnfig map müssen wir wie secrets auslesen. Die Referenz muss in den Specs des baremetal machine objects stehen.
	TODO: Wie kann man das mit go templates machen?
	Es wäre sinnvoll, die Config map als erstes zu validieren, um direkt in einen error state gehen zu können, ohne die aufwändigen Sachen
	vom Anfang zu machen. Validierung heißt hier, dass wir die drei Variablen einsetzen können und templaten können. Wir wollen auch einen
	default setzen (als String in den Code), der genutzt wird, wenn keine Referenz auf eine Config map gegeben ist. Wenn eine Referenz da ist,
	aber die config map nicht gefunden wurde, dann geben wir einen error aus.
	5. autosetup per SSH übertragen/die Datei auf dem Server erstellen
	6. ssh command installimage ausführen. Dann warten. Wenn es erfolgreich war, gehe zu StateImageInstalledSSHReboot.
	TODO: Wie kann man auch neu reconcilen und irgendwie herausfinden, ob installimage durchgelaufen ist?
6. StateImageInstalledSSHReboot: SSH reboot. Wenn wir dann per SSH Verbindung bekommen und hostname != rescue ist, dann hat es funktioniert und wir
gehen einen State weiter. Wenn hostname = rescue ist, dann probiere es 30 sek lang und wenn immer noch hostname = rescue ist, dann gehe zu
StateImageInstalledAPIReboot. Wenn man keine SSH Verbindung bekommt, dann überprüfe nach 3 min ob der Server eingeschaltet ist. Wenn nicht,
gehe in den StateImageInstalledSSHRebootPowerOn. Wenn er eingeschaltet ist, dann gehe nach 5 Minuten zu StateImageInstalledAPIReboot. Wenn es
funktioniert, dann gehe zu StateCloudInit.
6a. StateImageInstalledSSHRebootPowerOn: Versuche 5 min lang den Server anzuschalten, wenn es nicht klappt, gehe zu
StateImageInstalledAPIReboot. Wenn es klappt, gehe zurück zu StateImageInstalledSSHReboot.
7. StateImageInstalledAPIReboot: Versuche 10 min lang per SSH zu verbinden. Wenn es nach 4 min nicht klappt, dann schaue ob der Server
ausgeschaltet ist. Wenn ja, gehe zu StateImageInstalledAPIRebootPowerOn. Wenn man immer hostname = rescue bekommt, dann mache einen
exponential backoff beim Überprüfen. Wenn es gar nicht klappt, dann gebe permanenten Error aus,
sodass wir von vorne anfangen. Wenn es klappt, gehe zu StateCloudInit.
7a. StateImageInstalledAPIRebootPowerOn: Versuche 5 min lang Server anzuschalten. Wenn es nicht klappt, fange von vorne an. Wenn es klappt,
gehe zu StateImageInstalledAPIReboot bzw. einem neuen State .

8. StateImageInstalledPowerOn: Server wird per API /reset/server_id und type=power eingeschaltet.
9. StateCloudInit: Wenn er nicht auf auf host machine ist, dann gehe zurück zu reboot states. Wenn Cloud Init nicht installiert ist,
(command -v <the_command>) dann event und error -> state available. Danach machen wir mehrere Kommandos und übertragen Bootstrap Data.
Alle Befehle können immer wieder von Anfang an durchgegangen werden. DANACH REBOOT LOGIK.



StateImageInstalledSSHReboot:
Versuche mit SSH auf die maschine zu kommen. Klappt es?
	Ja:
		Ist hostname != rescue:
			Ja:
				Gehe zu StateCloudInit.
			Nein:
				Ist ErrorRebootNotTriggered vor mehr als 30 sek gesetzt?
					Ja:
						Gehe zu StateImageInstalledAPIReboot
					Nein:
						Setze error state ErrorRebootNotTriggered
	Nein:
		Ist die Maschine auf power on?
		Ja:
			Ist ErrorRebootOverdue seit mind. 3 min gesetzt?
				Ja:
					Gehe zu StateImageInstalledAPIReboot
				Nein:
					Setze ErrorRebootOverdue
		Nein:
			Gehe zu StateImageInstalledSSHRebootPowerOn

StateImageInstalledSSHRebootPowerOn:
Schaue ob Maschine eingeschaltet ist.
	Ja:
		Gehe in StateImageInstalledSSHRebootPowerOnReboot.
	Nein:
		Ist der Error state ErrorPowerOnNotSuccessfull länger als 5 min?
			Ja:
				Gehe zu StateImageInstalledAPIReboot
			Nein:
				Setze error.

StateImageInstalledSSHRebootPowerOnReboot:
Das gleiche wie oben, nur ohne Überprüfung ob Maschine auf power on ist.



Notiz:
- Wenn eine baremetal machine hosts auflistet um einen auszuwählen, dann soll sie ein event emitieren, wenn sie hosts findet, die auf die
gleiche Baremetal machine bei Hetzner zeigen. Diese werden nicht ausgewählt.
- Wir erlauben KEINE Änderungen bei den Specs des Hosts
- Wir brauchen Logik, dass man bei einem Host sagen kann, dass man ihn loswerden will und dass dies entsprechend von der baremetal machine
erkannt wird -> maintenance mode. Dieser Modus friert den derzeitigen Status ein, oder setzt ihn zurück, wenn gerade etwas getan wird (wie
provisionieren). Wenn er provisioniert ist, soll er deprovisioniert werden.

SSH key management:

in HetznerCluster reconcile:

1. Validierung in webhooks bei SSHKey -> mind. len(HCloud) >0 oder HRobot != nil
2. Überprüfe, ob sich Resource Version des Secrets geändert hat (falls RobotSSHKey Status noch nicht gesetzt wird)
3. Wenn ja, dann fange von vorne an:
	1. prüfe ob Public und private key im Secret matchen (https://stackoverflow.com/questions/64860094/how-to-verify-if-a-public-key-matched-private-key-signature) RSA sind ECDSA und ED25519 https://docs.hetzner.com/de/robot/dedicated-server/security/ssh/
	2. Check if key exists und ob der public key mit unserem übereinstimmt (entweder durch Vergleich von data oder über fingerprint)
		-> schreibe das in den Status
	3. Uploade key (wenn nichts im Status ist)

in Host Reconcile:
- Überprüfe ob Resource Version sich geändert hat, oder ob SSHKey im Cluster Status anders ist als der im eigenen Status.
Wenn ja, dann provisioniere von vorne, wenn man gerade dabei ist

Robot Credentials test:
1. Überprüfe in https://robot-ws.your-server.de/ ob es einen 401 error gibt
-> Überprüfe auch hier die Resource Version des Secrets und wenn es sich geändert hat, dann überprüfe es noch einmal
*/
