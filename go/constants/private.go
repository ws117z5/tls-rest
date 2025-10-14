package constants

var (
	//openssl rand -base64 32
	JWTSignature = []byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")

	VKID     = 6606830
	VKSecKey = "Wrd1ZEEN6liI8FfsviFG"
	VKLink   = "https://localhost:8080/users/Auth/Vk"

	GoogleID = "168149043882-i1mdro45fm73lkd0b6homtq6jm0ojrll.apps.googleusercontent.com"
	//GoogleID       = "168149043882-lp9s7uck2p29u17p63258botl1r0bliu.apps.googleusercontent.com"
	GoogleSecret = "JHHEGldk1N7YaXEGfkSlQCYK"
	//GoogleSecret   = "_JhnmJsono2Q-_Yt1ffkAo6B"
	GoogleURLBlank = "urn:ietf:wg:oauth:2.0:oob"
	GoogleURLLocal = "https://localhost/users/Auth/Google"

	PDbAddr = "postgres://ws117z5:monkeybusiness@localhost/tls-rest?sslmode=verify-full"
	PdbUser = "ws117z5"
	PdbPass = "monkeybusiness"
)
