package com.mtlog.analyzer.listener

import com.goide.psi.GoFile
import com.intellij.codeInsight.daemon.DaemonCodeAnalyzer
import com.intellij.openapi.editor.event.DocumentEvent
import com.intellij.openapi.editor.event.DocumentListener
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.Key
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiFile
import com.mtlog.analyzer.logging.MtlogLogger
import com.intellij.openapi.application.ApplicationManager

/**
 * Document listener that forces re-analysis after undo operations.
 */
class MtlogDocumentListener(private val project: Project) : DocumentListener {
    
    companion object {
        private val LAST_TEXT_LENGTH_KEY = Key.create<Int>("MTLOG_LAST_TEXT_LENGTH")
        private val ANALYSIS_SCHEDULED_KEY = Key.create<Boolean>("MTLOG_ANALYSIS_SCHEDULED")
    }
    
    override fun documentChanged(event: DocumentEvent) {
        val document = event.document
        val file = PsiDocumentManager.getInstance(project).getPsiFile(document) as? GoFile ?: return
        
        // Get the last known text length
        val lastLength = file.getUserData(LAST_TEXT_LENGTH_KEY) ?: -1
        val currentLength = document.textLength
        
        MtlogLogger.debug("Document changed: ${file.name}, lastLength=$lastLength, currentLength=$currentLength", project)
        
        // Check if this might be an undo operation (text length decreased)
        if (lastLength > currentLength && lastLength - currentLength > 3) {
            MtlogLogger.info("Possible undo detected in ${file.name}: length changed from $lastLength to $currentLength", project)
            
            // Check if we already scheduled analysis
            val analysisScheduled = file.getUserData(ANALYSIS_SCHEDULED_KEY) ?: false
            if (!analysisScheduled) {
                file.putUserData(ANALYSIS_SCHEDULED_KEY, true)
                
                // Schedule re-analysis with a small delay to let undo complete
                ApplicationManager.getApplication().invokeLater {
                    MtlogLogger.info("Forcing re-analysis after possible undo in ${file.name}", project)
                    
                    // First, commit the document to sync with PSI
                    PsiDocumentManager.getInstance(project).commitDocument(document)
                    
                    // Invalidate the PSI file to force a fresh view
                    val psiManager = file.manager
                    psiManager.dropPsiCaches()
                    
                    // Try multiple approaches to force refresh
                    val analyzer = DaemonCodeAnalyzer.getInstance(project)
                    
                    // First, restart just the file
                    analyzer.restart(file)
                    
                    // Force a re-parse by touching the PSI structure
                    ApplicationManager.getApplication().runWriteAction {
                        file.subtreeChanged()
                    }
                    
                    // Then schedule a full restart with delay
                    ApplicationManager.getApplication().invokeLater {
                        MtlogLogger.info("Forcing FULL daemon restart", project)
                        analyzer.restart()
                    }
                    
                    file.putUserData(ANALYSIS_SCHEDULED_KEY, false)
                }
            }
        }
        
        // Store current length for next comparison
        file.putUserData(LAST_TEXT_LENGTH_KEY, currentLength)
    }
}