package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	st "github.com/Yukhoi/PC3R_TME4/client/structures"

	// contient la structure Personne
	tr "github.com/Yukhoi/PC3R_TME4/client/travaux"
	// contient les fonctions de travail sur les Personnes
)

var ADRESSE string = "localhost"                           // adresse de base pour la Partie 2
var FICHIER_SOURCE string = "./conseillers-municipaux.txt" // fichier dans lequel piocher des personnes
var TAILLE_SOURCE int = 9                                  // inferieure au nombre de lignes du fichier, pour prendre une ligne au hasard
var TAILLE_G int = 5                                       // taille du tampon des gestionnaires
var NB_G int = 2                                           // nombre de gestionnaires
var NB_P int = 2                                           // nombre de producteurs
var NB_O int = 4                                           // nombre d'ouvriers
var NB_PD int = 2                                          // nombre de producteurs distants pour la Partie 2

var pers_vide = st.Personne{Nom: "", Prenom: "", Age: 0, Sexe: "M"} // une personne vide

type message_lec struct {
	ligne  int
	retour chan string
}

type message_dist struct {
	id      int
	retour  chan string // socket TCP
	methode string
}

// paquet de personne, sur lequel on peut travailler, implemente l'interface personne_int
type personne_emp struct {
	ligne    int
	statut   string
	afaire   []func(st.Personne) st.Personne
	personne st.Personne
	lecteur  chan message_lec
}

// paquet de personne distante, pour la Partie 2, implemente l'interface personne_int
type personne_dist struct {
	identifiant int
	proxy       chan message_dist
}

// interface des personnes manipulees par les ouvriers, les
type personne_int interface {
	initialise()          // appelle sur une personne vide de statut V, remplit les champs de la personne et passe son statut à R
	travaille()           // appelle sur une personne de statut R, travaille une fois sur la personne et passe son statut à C s'il n'y a plus de travail a faire
	vers_string() string  // convertit la personne en string
	donne_statut() string // renvoie V, R ou C
}

// fabrique une personne à partir d'une ligne du fichier des conseillers municipaux
// à changer si un autre fichier est utilisé
func personne_de_ligne(l string) st.Personne {
	separateur := regexp.MustCompile("\u0009") // oui, les donnees sont separees par des tabulations ... merci la Republique Francaise
	separation := separateur.Split(l, -1)
	naiss, _ := time.Parse("2/1/2006", separation[7])
	a1, _, _ := time.Now().Date()
	a2, _, _ := naiss.Date()
	agec := a1 - a2
	return st.Personne{Nom: separation[4], Prenom: separation[5], Sexe: separation[6], Age: agec}
}

// *** METHODES DE L'INTERFACE personne_int POUR LES PAQUETS DE PERSONNES ***

func (p *personne_emp) initialise() {
	ret := make(chan string)
	p.lecteur <- message_lec{ligne: p.ligne, retour: ret}
	contenu := <-ret

	//initialise personne
	p.personne = personne_de_ligne(contenu)

	//modifie le status
	for i := 0; i < rand.Intn(6); i++ {
		p.afaire = append(p.afaire, tr.UnTravail())
	}

	p.statut = "R"

	if len(p.afaire) == 0 {
		p.statut = "C"
	}

}

func (p *personne_emp) travaille() {
	if len(p.afaire) == 0 {
		p.statut = "C"
	}
	p.personne = p.afaire[0](p.personne)
	p.afaire = p.afaire[1:]

}

func (p *personne_emp) vers_string() string {
	var add string
	if p.personne.Sexe == "F" {
		add = "Mme "
	} else {
		add = "M "
	}
	return fmt.Sprint(add, p.personne.Prenom, " ", p.personne.Nom, " : ", p.personne.Age, " ans. ")
}

func (p *personne_emp) donne_statut() string {
	return p.statut
}

// *** METHODES DE L'INTERFACE personne_int POUR LES PAQUETS DE PERSONNES DISTANTES (PARTIE 2) ***
// ces méthodes doivent appeler le proxy (aucun calcul direct)

func (p personne_dist) initialise() {
	retour := make(chan string)
	message := message_dist{id: p.identifiant, methode: "initialise", retour: retour}
	p.proxy <- message
	//methode initialise: non valeur retournée
	<-retour
}

func (p personne_dist) travaille() {
	retour := make(chan string)
	message := message_dist{id: p.identifiant, methode: "travaille", retour: retour}
	p.proxy <- message
	//methode travaille: non valeur retournée
	<-retour
}

func (p personne_dist) vers_string() string {
	retour := make(chan string)
	message := message_dist{id: p.identifiant, methode: "vers_string", retour: retour}
	p.proxy <- message
	//methode vers_string: retourner un string
	return <-retour
}

func (p personne_dist) donne_statut() string {
	retour := make(chan string)
	message := message_dist{id: p.identifiant, methode: "donne_statut", retour: retour}
	p.proxy <- message
	//methode vers_string: retourner un string
	return <-retour
}

// *** CODE DES GOROUTINES DU SYSTEME ***

// Partie 2: contacté par les méthodes de personne_dist, le proxy appelle la méthode à travers le réseau et récupère le résultat
// il doit utiliser une connection TCP sur le port donné en ligne de commande
func proxy(port string, requete chan message_dist) {
	address := ADRESSE + ":" + port
	conn, _ := net.Dial("tcp", address)
	for {
		message := <-requete
		requete := strconv.Itoa(message.id) + "," + message.methode + "\n"
		fmt.Fprintf(conn, fmt.Sprintf(requete))
		recu, _ := bufio.NewReader(conn).ReadString('\n')
		reponse := strings.TrimSuffix(recu, "\n")
		fmt.Println("Réponse reçu du serveur: " + reponse)
		message.retour <- reponse
	}
	conn.Close()
}

// Partie 1 : contacté par la méthode initialise() de personne_emp, récupère une ligne donnée dans le fichier source
func lecteur(lectureC chan message_lec) {
	for {
		m := <-lectureC
		fmt.Println("lecteur" + strconv.Itoa(m.ligne))
		file, err := os.Open(FICHIER_SOURCE)
		if err != nil {
			log.Fatal(err)
		}
		scanner := bufio.NewScanner(file)

		//sauter la première ligne
		_ = scanner.Scan()

		//sauter les premières m.ligne-ième lignes
		for i := 0; i < m.ligne; i++ {
			_ = scanner.Scan()
		}
		result := scanner.Scan()
		if !result {
			log.Fatal(err)
		} else {
			m.retour <- scanner.Text()
		}
		_ = file.Close()
	}
}

// Partie 1: récupèrent des personne_int depuis les gestionnaires, font une opération dépendant de donne_statut()
// Si le statut est V, ils initialise le paquet de personne puis le repasse aux gestionnaires
// Si le statut est R, ils travaille une fois sur le paquet puis le repasse aux gestionnaires
// Si le statut est C, ils passent le paquet au collecteur
func ouvrier(fromGest chan personne_int, toGest chan personne_int, toCollec chan personne_int) {
	for {
		personne := <-fromGest
		if personne.donne_statut() == "V" {
			personne.initialise()
			toGest <- personne
		} else if personne.donne_statut() == "R" {
			personne.travaille()
			toGest <- personne
		} else {
			toCollec <- personne
		}
	}
}

// Partie 1: les producteurs cree des personne_int implementees par des personne_emp initialement vides,
// de statut V mais contenant un numéro de ligne (pour etre initialisee depuis le fichier texte)
// la personne est passée aux gestionnaires
func producteur(lecture chan message_lec, prod chan personne_int) {
	for {
		personne := personne_emp{ligne: rand.Intn(TAILLE_SOURCE), personne: pers_vide, afaire: make([]func(st.Personne) st.Personne, 0), statut: "V", lecteur: lecture}
		prod <- personne_int(&personne)
		fmt.Println("product")
	}
}

// Partie 2: les producteurs distants cree des personne_int implementees par des personne_dist qui contiennent un identifiant unique
// utilisé pour retrouver l'object sur le serveur
// la creation sur le client d'une personne_dist doit declencher la creation sur le serveur d'une "vraie" personne, initialement vide, de statut V
func producteur_distant(deProdVersGest chan personne_int, requeteChan chan message_dist, id_frais_chan chan int) {
	for {
		id := <-id_frais_chan
		new_pers := personne_dist{identifiant: id, proxy: requeteChan}
		retour := make(chan string)
		requeteChan <- message_dist{id: id, methode: "creer", retour: retour}
		// prod_distant -> proxy -> serveur -> proxy --(retour)--> prod_distant
		<-retour
		// envoyer cette personne vers gestionnaire
		deProdVersGest <- new_pers
	}
}

// Partie 1: les gestionnaires recoivent des personne_int des producteurs et des ouvriers et maintiennent chacun une file de personne_int
// ils les passent aux ouvriers quand ils sont disponibles
// ATTENTION: la famine des ouvriers doit être évitée: si les producteurs inondent les gestionnaires de paquets, les ouvrier ne pourront
// plus rendre les paquets surlesquels ils travaillent pour en prendre des autres
func gestionnaire(fromProd chan personne_int, toOuvrier chan personne_int, fromOuvrier chan personne_int) {
	queue := make([]personne_int, 0)
	for {
		if len(queue) == TAILLE_G {
			//full
			toOuvrier <- queue[0]
			queue = queue[1:]
			fmt.Println("ges give to ouv")
		} else if len(queue) == 0 {
			//vide
			select {
			case personne := <-fromProd:
				queue = append(queue, personne)
				fmt.Println("ges recieve from prod")
			case personne := <-fromOuvrier:
				queue = append(queue, personne)
				fmt.Println("ges recieve from ouv")

			}
		} else if len(queue) < TAILLE_G-1 {
			//2 places pour paquet de ouvrier
			select {
			case personne := <-fromProd:
				queue = append(queue, personne)
				//fmt.Println("ges recieve from prod")
			case personne := <-fromOuvrier:
				queue = append(queue, personne)
				//fmt.Println("ges recieve from ouv")
			case toOuvrier <- queue[0]:
				queue = queue[1:]
				//fmt.Println("ges give to ouv")
			}
		} else {
			//pas assez de places pour paquet de producteur
			select {
			case personne := <-fromOuvrier:
				queue = append(queue, personne)
				//fmt.Println("ges recieve from ouv")
			case fromOuvrier <- queue[0]:
				queue = queue[1:]
				//fmt.Println("ges give to ouv")

			}
		}
	}
}

// Partie 1: le collecteur recoit des personne_int dont le statut est c, il les collecte dans un journal
// quand il recoit un signal de fin du temps, il imprime son journal.
func collecteur(toCollecteur chan personne_int, mainChan chan int) {
	var journal string
	for {
		select {
		case personne := <-toCollecteur:
			journal += personne.vers_string() + "\n"
		case <-mainChan:
			fmt.Println("Journal:\n" + journal)
			mainChan <- 0
			return
		}
	}
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano()) // graine pour l'aleatoire

	if len(os.Args) < 3 {
		fmt.Println("Format: client <port> <millisecondes d'attente>")
		return
	}

	port := os.Args[1]                    // utile pour la partie 2
	millis, _ := strconv.Atoi(os.Args[2]) // duree du timeout
	fintemps := make(chan int)

	// creer les canaux
	lecture := make(chan message_lec)
	fromProdToGest := make(chan personne_int)
	fromGestToOuvrier := make(chan personne_int)
	fromOuvrierToGest := make(chan personne_int)
	fromOuvrierToCollec := make(chan personne_int)

	requete := make(chan message_dist)
	id_frais_chan := make(chan int)

	// lancer les goroutines (parties 1 et 2): 1 lecteur, 1 collecteur, des producteurs, des gestionnaires, des ouvriers
	go func() {
		lecteur(lecture)
	}()
	go func() {
		collecteur(fromOuvrierToCollec, fintemps)
	}()
	for i := 0; i < NB_P; i++ {
		go func() {
			producteur(lecture, fromProdToGest)
		}()
	}
	for i := 0; i < NB_G; i++ {
		go func() {
			gestionnaire(fromProdToGest, fromGestToOuvrier, fromOuvrierToGest)
		}()
	}
	for i := 0; i < NB_O; i++ {
		go func() {
			ouvrier(fromGestToOuvrier, fromOuvrierToGest, fromOuvrierToCollec)
		}()
	}
	//lancer les goroutines (partie 2): des producteurs distants, un proxy
	go func() {
		proxy(port, requete)
	}()
	go func() {
		compteur := 0
		for {
			id_frais_chan <- compteur
			compteur++
		}
	}()
	for i := 0; i < NB_PD; i++ {
		go func() {
			producteur_distant(fromProdToGest, requete, id_frais_chan)
		}()
	}

	time.Sleep(time.Duration(millis) * time.Millisecond)
	fintemps <- 0
	<-fintemps
}
