plugins {
    kotlin("jvm") version "1.9.22"
    application
}

group = "dev.feather"
version = "1.0.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation("dev.feather:feather-client:1.0.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.8.0")
}

kotlin {
    jvmToolchain(17)
}

application {
    mainClass.set("dev.feather.quickstart.QuickstartKt")
}

tasks.jar {
    manifest {
        attributes["Main-Class"] = "dev.feather.quickstart.QuickstartKt"
    }
    from(configurations.runtimeClasspath.get().map { if (it.isDirectory) it else zipTree(it) })
    duplicatesStrategy = DuplicatesStrategy.EXCLUDE
}
