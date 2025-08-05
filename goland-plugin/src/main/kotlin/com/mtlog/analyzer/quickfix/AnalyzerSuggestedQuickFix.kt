package com.mtlog.analyzer.quickfix

import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.mtlog.analyzer.logging.MtlogLogger
import com.mtlog.analyzer.service.AnalyzerSuggestedFix
import com.mtlog.analyzer.service.AnalyzerTextEdit

/**
 * Quick fix that applies text edits suggested by the mtlog-analyzer.
 * This provides parity with the VS Code extension behavior.
 */
class AnalyzerSuggestedQuickFix(
    element: PsiElement?,
    private val suggestedFix: AnalyzerSuggestedFix
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    override fun getText(): String = suggestedFix.message
    
    override fun getFamilyName(): String = "mtlog"
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        MtlogLogger.info("===== AnalyzerSuggestedQuickFix.invoke CALLED =====", project)
        MtlogLogger.info("Fix: ${suggestedFix.message}", project)
        MtlogLogger.info("Text edits: ${suggestedFix.textEdits.size}", project)
        
        val doc = editor?.document 
            ?: PsiDocumentManager.getInstance(project).getDocument(file) 
            ?: return
        
        val runnable = Runnable {
            try {
                // Apply text edits in reverse order (from end to start) to maintain correct offsets
                val sortedEdits = suggestedFix.textEdits.sortedByDescending { edit ->
                    try {
                        val pos = parsePosition(edit.pos)
                        val lineStartOffset = doc.getLineStartOffset(pos.line - 1)
                        lineStartOffset + (pos.column - 1)
                    } catch (e: Exception) {
                        // If position is invalid, sort it last
                        -1
                    }
                }
                
                for (edit in sortedEdits) {
                    val startPos = parsePosition(edit.pos)
                    val endPos = parsePosition(edit.end)
                    
                    MtlogLogger.debug("Applying edit: pos=${edit.pos}, end=${edit.end}, newText='${edit.newText}'", project)
                    
                    // Convert line:column to document offset
                    val startOffset = try {
                        val lineStartOffset = doc.getLineStartOffset(startPos.line - 1)
                        lineStartOffset + (startPos.column - 1)
                    } catch (e: Exception) {
                        MtlogLogger.warn("Invalid start position ${edit.pos}: ${e.message}", project)
                        continue
                    }
                    
                    val endOffset = try {
                        val lineStartOffset = doc.getLineStartOffset(endPos.line - 1)
                        lineStartOffset + (endPos.column - 1)
                    } catch (e: Exception) {
                        MtlogLogger.warn("Invalid end position ${edit.end}: ${e.message}", project)
                        continue
                    }
                    
                    // Validate offsets
                    if (startOffset < 0 || endOffset > doc.textLength || startOffset > endOffset) {
                        MtlogLogger.warn("Invalid offsets: start=$startOffset, end=$endOffset, docLength=${doc.textLength}", project)
                        continue
                    }
                    
                    // Apply the edit
                    if (endOffset > startOffset) {
                        // Replace text
                        doc.replaceString(startOffset, endOffset, edit.newText)
                    } else {
                        // Insert text
                        doc.insertString(startOffset, edit.newText)
                    }
                    
                    MtlogLogger.debug("Applied edit successfully", project)
                }
                
                // Commit document changes
                PsiDocumentManager.getInstance(project).commitDocument(doc)
                
                // Save file and force re-analysis
                ApplicationManager.getApplication().invokeLater {
                    doc.let {
                        val fileDocManager = com.intellij.openapi.fileEditor.FileDocumentManager.getInstance()
                        fileDocManager.saveDocument(it)
                    }
                    
                    com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart(file)
                }
                
            } catch (e: Exception) {
                MtlogLogger.error("Error applying analyzer suggested fix", project, e)
            }
        }
        
        // Always wrap in WriteCommandAction
        if (!ApplicationManager.getApplication().isWriteAccessAllowed) {
            WriteCommandAction.runWriteCommandAction(project, suggestedFix.message, null, runnable, file)
        } else {
            runnable.run()
        }
    }
    
    private data class Position(val line: Int, val column: Int)
    
    private fun parsePosition(posStr: String): Position {
        // Parse format: "file:line:col"
        val parts = posStr.split(":")
        if (parts.size >= 3) {
            val line = parts[parts.size - 2].toIntOrNull() ?: 1
            val column = parts[parts.size - 1].toIntOrNull() ?: 1
            return Position(line, column)
        }
        return Position(1, 1)
    }
}