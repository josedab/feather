# Feather Java/Kotlin Quickstart

Get started with Feather in 30 seconds.

## Prerequisites

- JDK 17+
- Docker (for running Feather server)
- Gradle or Maven

## Step 1: Start Feather

```bash
# From source (no Docker needed)
cd /path/to/feather && make run-dev

# Or with Docker
docker run -d --name feather -p 8080:8080 ghcr.io/feather-store/feather:latest
```

## Step 2: Run the Quickstart

With Gradle:
```bash
./gradlew run
```

Or compile and run manually:
```bash
./gradlew build
java -jar build/libs/quickstart.jar
```

## What This Does

1. Connects to Feather
2. Stores features for a user entity
3. Retrieves the features back
4. Demonstrates batch retrieval

## Using with Maven

Add to your `pom.xml`:
```xml
<dependency>
    <groupId>dev.feather</groupId>
    <artifactId>feather-client</artifactId>
    <version>1.0.0</version>
</dependency>
```

## Next Steps

- Check out the [full documentation](https://feather-store.dev/docs)
- Explore [vector similarity search](https://feather-store.dev/docs/vectors)
- Learn about [Spring Boot integration](https://feather-store.dev/docs/integrations/spring)
