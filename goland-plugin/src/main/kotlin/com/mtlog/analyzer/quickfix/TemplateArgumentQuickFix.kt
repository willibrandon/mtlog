package com.mtlog.analyzer.quickfix

import com.goide.psi.GoCallExpr
import com.goide.psi.GoStringLiteral
import com.goide.psi.impl.GoElementFactory
import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.util.PsiTreeUtil
import com.mtlog.analyzer.MtlogBundle
import com.mtlog.analyzer.logging.MtlogLogger

/**
 * Quick fix to match template properties with arguments.
 */
class TemplateArgumentQuickFix(
    element: PsiElement? = null
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    init {
        element?.project?.let { project ->
            MtlogLogger.info("TemplateArgumentQuickFix CREATED", project)
        }
    }
    
    override fun getText(): String {
        MtlogLogger.info("TemplateArgumentQuickFix.getText() called", startElement?.project)
        return "Fix template arguments"
    }
    
    override fun getFamilyName(): String = MtlogBundle.message("quickfix.family.name")
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        MtlogLogger.info("===== TemplateArgumentQuickFix.invoke CALLED =====", project)
        MtlogLogger.info("startElement: ${startElement.javaClass.simpleName}, text: '${startElement.text.take(50)}'...", project)
        MtlogLogger.info("file: ${file.name}", project)
        // Find the string literal and call expression
        MtlogLogger.info("Looking for GoStringLiteral...", project)
        
        // First try to find string literal within the element (if startElement is a call expression)
        val goStringLiteral = if (startElement is GoCallExpr) {
            MtlogLogger.info("startElement is GoCallExpr, looking for string literal inside it", project)
            PsiTreeUtil.findChildOfType(startElement, GoStringLiteral::class.java)
        } else {
            MtlogLogger.info("startElement is not GoCallExpr, traversing up", project)
            var stringLiteral: PsiElement? = startElement
            while (stringLiteral != null && stringLiteral !is GoStringLiteral) {
                stringLiteral = stringLiteral.parent
            }
            stringLiteral as? GoStringLiteral
        }
        
        if (goStringLiteral == null) {
            MtlogLogger.error("Could not find GoStringLiteral", project)
            return
        }
        MtlogLogger.info("Found GoStringLiteral: ${goStringLiteral.text}", project)
        
        val callExpr = if (startElement is GoCallExpr) {
            startElement
        } else {
            PsiTreeUtil.getParentOfType(goStringLiteral, GoCallExpr::class.java)
        }
        
        if (callExpr == null) {
            MtlogLogger.error("Could not find GoCallExpr", project)
            return
        }
        MtlogLogger.info("Found GoCallExpr: ${callExpr.text.take(50)}...", project)
        
        val doc = editor?.document 
            ?: PsiDocumentManager.getInstance(project).getDocument(file) 
            ?: return
        
        // Get template text and count properties
        val templateText = goStringLiteral.text
        val templateContent = when {
            templateText.startsWith("\"") && templateText.endsWith("\"") -> 
                templateText.substring(1, templateText.length - 1)
            templateText.startsWith("`") && templateText.endsWith("`") -> 
                templateText.substring(1, templateText.length - 1)
            else -> return
        }
        
        // Count properties in template (anything in {})
        val propertyCount = "\\{[^}]+\\}".toRegex().findAll(templateContent).count()
        MtlogLogger.info("Template has $propertyCount properties", project)
        
        // Get current argument count (excluding the template string itself)
        val argList = callExpr.argumentList?.expressionList ?: emptyList()
        val currentArgCount = argList.size - 1 // Subtract template string
        MtlogLogger.info("Current argument count: $currentArgCount (total args: ${argList.size})", project)
        
        if (propertyCount == currentArgCount) {
            MtlogLogger.info("Argument count already correct, nothing to do", project)
            return
        }
        
        // The actual modification logic
        MtlogLogger.info("Creating runnable for modifications...", project)
        val runnable = Runnable {
            MtlogLogger.info("Inside runnable, about to make changes", project)
            try {
                when {
                    propertyCount > currentArgCount -> {
                        // Add missing arguments
                        val lastArg = argList.lastOrNull() ?: return@Runnable
                        val insertPos = lastArg.textRange.endOffset
                        val missingCount = propertyCount - currentArgCount
                        
                        // Check if template contains {Error} or {Err} and if this is an Error method
                        val hasErrorPlaceholder = templateContent.contains("{Error}") || templateContent.contains("{Err}")
                        val methodName = callExpr.expression?.text?.substringAfterLast('.') ?: ""
                        val isErrorMethod = methodName == "Error" || methodName == "E"
                        
                        // Always use nil for all missing arguments to avoid undefined variable errors
                        val argsToAdd = List(missingCount) { "nil" }.joinToString(", ", ", ")
                        
                        doc.insertString(insertPos, argsToAdd)
                    }
                    propertyCount < currentArgCount -> {
                        // Remove extra arguments
                        val extraCount = currentArgCount - propertyCount
                        val argsToRemove = argList.takeLast(extraCount)
                        
                        if (argsToRemove.isNotEmpty()) {
                            val firstToRemove = argsToRemove.first()
                            val lastToRemove = argsToRemove.last()
                            
                            // Find the comma before the first argument to remove
                            var deleteStart = firstToRemove.textRange.startOffset
                            var searchPos = deleteStart - 1
                            while (searchPos >= 0 && doc.charsSequence[searchPos].isWhitespace()) {
                                searchPos--
                            }
                            if (searchPos >= 0 && doc.charsSequence[searchPos] == ',') {
                                deleteStart = searchPos
                            }
                            
                            val deleteEnd = lastToRemove.textRange.endOffset
                            doc.deleteString(deleteStart, deleteEnd)
                        }
                    }
                }
                
                MtlogLogger.info("Document changes committed successfully", project)
                PsiDocumentManager.getInstance(project).commitDocument(doc)
                
                // Save the file to disk so external analyzer sees the changes
                MtlogLogger.info("Saving file to disk", project)
                com.intellij.openapi.fileEditor.FileDocumentManager.getInstance().saveDocument(doc)
                
                // Force re-analysis of the file
                MtlogLogger.info("Forcing re-analysis of file", project)
                com.intellij.codeInsight.daemon.DaemonCodeAnalyzer.getInstance(project).restart(file)
            } catch (e: Exception) {
                MtlogLogger.error("Error in TemplateArgumentQuickFix", project, e)
                e.printStackTrace()
            }
        }
        
        executeWithAppropriateWriteAction(project, file, runnable)
    }
    
    /**
     * Executes the given runnable with the appropriate write action context.
     * Always wraps in WriteCommandAction if not already in a write action to avoid threading issues.
     */
    private fun executeWithAppropriateWriteAction(project: Project, file: PsiFile, runnable: Runnable) {
        val app = ApplicationManager.getApplication()
        MtlogLogger.info("executeWithAppropriateWriteAction: writeAccessAllowed=${app.isWriteAccessAllowed}", project)
        
        // Always wrap in WriteCommandAction if not already in write action
        if (!app.isWriteAccessAllowed) {
            WriteCommandAction.runWriteCommandAction(project, getText(), null, runnable, file)
        } else {
            runnable.run()
        }
    }
}