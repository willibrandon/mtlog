package com.mtlog.analyzer.quickfix

import com.goide.psi.GoCallExpr
import com.goide.psi.GoStringLiteral
import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.diagnostic.logger
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.util.PsiTreeUtil
import com.mtlog.analyzer.notification.MtlogNotificationService

/**
 * Quick fix to add missing error parameter when template contains {Error} or {Err}.
 */
class MissingErrorQuickFix(
    element: PsiElement? = null
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    companion object {
        private val LOG = logger<MissingErrorQuickFix>()
    }
    
    override fun getText(): String = "Add {Error} to template and nil parameter"
    
    override fun getFamilyName(): String = "mtlog"
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        LOG.debug("MissingErrorQuickFix.invoke called with startElement: ${startElement.javaClass.simpleName}, text: '${startElement.text}'")
        
        // Find the call expression first
        val callExpr = when {
            startElement is GoCallExpr -> startElement
            else -> PsiTreeUtil.getParentOfType(startElement, GoCallExpr::class.java)
        }
        
        if (callExpr == null) {
            LOG.warn("Could not find GoCallExpr from startElement: ${startElement.javaClass.simpleName}")
            return
        }
        
        LOG.debug("Found call expression: ${callExpr.text}")
        
        // Find the string literal within the call expression
        val goStringLiteral = PsiTreeUtil.findChildOfType(callExpr, GoStringLiteral::class.java)
        if (goStringLiteral == null) {
            LOG.warn("Could not find GoStringLiteral in call expression")
            return
        }
        
        val doc = editor?.document 
            ?: PsiDocumentManager.getInstance(project).getDocument(file) 
            ?: return
        
        // Get template text
        val templateText = goStringLiteral.text
        val templateContent = when {
            templateText.startsWith("\"") && templateText.endsWith("\"") -> 
                templateText.substring(1, templateText.length - 1)
            templateText.startsWith("`") && templateText.endsWith("`") -> 
                templateText.substring(1, templateText.length - 1)
            else -> return
        }
        
        LOG.debug("Template content: '$templateContent'")
        
        // The actual modification logic
        val runnable = Runnable {
            try {
                // For MTLOG006, we always add {Error} to the template and nil parameter
                // MTLOG006 is only triggered when there's no error value in the log
                
                // Add {Error} to template
                LOG.debug("Adding {Error} to template and nil parameter")
                val templateEnd = goStringLiteral.textRange.endOffset - 1 // Before closing quote
                
                val errorPlaceholder = when {
                    templateContent.endsWith(".") || templateContent.endsWith("!") || templateContent.endsWith("?") -> " {Error}"
                    else -> ": {Error}"
                }
                
                LOG.debug("Inserting '$errorPlaceholder' at position $templateEnd")
                doc.insertString(templateEnd, errorPlaceholder)
                
                // Then add nil parameter at the end
                val argList = callExpr.argumentList
                val expressions = argList.expressionList
                if (expressions.isEmpty()) {
                    LOG.warn("Argument list is empty (only template), cannot determine where to add nil")
                    return@Runnable
                }
                
                // For MTLOG006, there's typically only the template string as an argument
                // We need to add ", nil" after the template string
                val templateArg = expressions.firstOrNull()
                if (templateArg == null) {
                    LOG.warn("Could not find template argument")
                    return@Runnable
                }
                
                val insertPos = templateArg.textRange.endOffset
                LOG.debug("Inserting ', nil' at position $insertPos after template")
                doc.insertString(insertPos, ", nil")
                
                LOG.debug("Committing document changes")
                PsiDocumentManager.getInstance(project).commitDocument(doc)
            } catch (e: Exception) {
                LOG.error("Error applying quick fix", e)
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
        LOG.debug("executeWithAppropriateWriteAction: writeAccessAllowed=${app.isWriteAccessAllowed}, readAccessAllowed=${app.isReadAccessAllowed}")
        
        // Always wrap in WriteCommandAction if not already in write action
        if (!app.isWriteAccessAllowed) {
            WriteCommandAction.runWriteCommandAction(project, getText(), null, runnable, file)
        } else {
            runnable.run()
        }
    }
}