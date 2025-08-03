import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    kotlin("jvm") version "2.0.0"
    id("org.jetbrains.intellij.platform") version "2.2.1"
}

group = "com.mtlog"
version = "0.7.0"

repositories {
    mavenCentral()
    
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        goland("2024.2")
        bundledPlugin("org.jetbrains.plugins.go")
        
        pluginVerifier()
        zipSigner()
        testFramework(TestFrameworkType.Platform)
    }
    
    implementation("com.google.code.gson:gson:2.10.1")
    
    testImplementation(kotlin("test"))
    testImplementation("org.opentest4j:opentest4j:1.3.0")
}

kotlin {
    jvmToolchain(21)
}

intellijPlatform {
    pluginConfiguration {
        name = "mtlog-analyzer"
        version = project.version.toString()
        
        ideaVersion {
            sinceBuild = "242"
            untilBuild = "252.*"
        }
        
        vendor {
            name = "mtlog"
            email = ""
            url = "https://github.com/willibrandon/mtlog"
        }
        
        description = """
            Real-time validation for mtlog message templates in Go code.
            
            Features:
            • Automatic template/argument validation
            • Three severity levels: errors, warnings, suggestions
            • Quick fixes for common issues
            • Configurable analyzer path and flags
            
            Requires mtlog-analyzer to be installed:
            go install github.com/willibrandon/mtlog/cmd/mtlog-analyzer@latest
        """.trimIndent()
        
        changeNotes = """
            <h2>0.7.0</h2>
            <ul>
                <li>Initial release</li>
                <li>Real-time template validation with proper text highlighting</li>
                <li>Quick fixes for property naming and argument count</li>
                <li>Configurable analyzer path and severity mappings</li>
                <li>Support for Windows, macOS, and Linux</li>
                <li>Integration with go vet</li>
            </ul>
        """.trimIndent()
    }
    
    signing {
        certificateChain = providers.environmentVariable("CERTIFICATE_CHAIN")
        privateKey = providers.environmentVariable("PRIVATE_KEY")
        password = providers.environmentVariable("PRIVATE_KEY_PASSWORD")
    }
    
    publishing {
        token = providers.environmentVariable("JETBRAINS_TOKEN")
    }
    
    pluginVerification {
        ides {
            ide("GO", "2024.2")
            ide("GO", "2025.1")
        }
    }
}

tasks {
    buildSearchableOptions {
        enabled = false  // Disable for initial development
    }
    
    instrumentCode {
        enabled = false  // Disable due to Java path issues
    }
    
    instrumentTestCode {
        enabled = false  // Disable due to Java path issues
    }
}
