import org.jetbrains.intellij.platform.gradle.TestFrameworkType

plugins {
    kotlin("jvm") version "2.0.0"
    id("org.jetbrains.intellij.platform") version "2.2.1"
}

group = "com.mtlog"
version = "0.7.3"

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
            <h2>0.7.3</h2>
            <ul>
                <li>Centralized all quick fixes in the analyzer for consistency</li>
                <li>Transitioned to stdin-based communication for real-time analysis</li>
                <li>Removed ~1000 lines of duplicate quick fix code</li>
                <li>Improved performance by eliminating file I/O operations</li>
                <li>Fixed stability issues with stdin mode</li>
                <li>Fixed IntelliJ IDEA Ultimate compatibility issue</li>
            </ul>
            <h2>0.7.2</h2>
            <ul>
                <li>Added diagnostic kill switch for quick enable/disable</li>
                <li>Added status bar widget with visual state indicator</li>
                <li>Added notification bar with Disable/Settings actions</li>
                <li>Added diagnostic suppression with immediate UI updates</li>
                <li>Fixed critical deadlock in template argument quick fix</li>
                <li>Inspection now enabled by default</li>
            </ul>
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
            ide("IU", "2024.2")  // IntelliJ IDEA Ultimate - Go plugin dependency resolved via bundledPlugin()
            ide("IU", "2025.1")
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
