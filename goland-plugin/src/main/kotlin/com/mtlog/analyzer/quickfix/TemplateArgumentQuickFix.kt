package com.mtlog.analyzer.quickfix

import com.goide.psi.GoCallExpr
import com.goide.psi.GoStringLiteral
import com.goide.psi.impl.GoElementFactory
import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.util.PsiTreeUtil
import com.mtlog.analyzer.MtlogBundle

/**
 * Quick fix to match template properties with arguments.
 */
class TemplateArgumentQuickFix(
    element: PsiElement? = null
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    override fun getText(): String = MtlogBundle.message("quickfix.template.arguments.name")
    
    override fun getFamilyName(): String = MtlogBundle.message("quickfix.family.name")
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        // Find the string literal and call expression
        var stringLiteral: PsiElement? = startElement
        while (stringLiteral != null && stringLiteral !is GoStringLiteral) {
            stringLiteral = stringLiteral.parent
        }
        val goStringLiteral = stringLiteral as? GoStringLiteral ?: return
        
        val callExpr = PsiTreeUtil.getParentOfType(goStringLiteral, GoCallExpr::class.java) ?: return
        
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
        
        // Get current argument count (excluding the template string itself)
        val argList = callExpr.argumentList?.expressionList ?: emptyList()
        val currentArgCount = argList.size - 1 // Subtract template string
        
        if (propertyCount == currentArgCount) return // Already correct
        
        // The actual modification logic
        val runnable = Runnable {
            when {
                propertyCount > currentArgCount -> {
                    // Add missing nil arguments
                    val lastArg = argList.lastOrNull() ?: return@Runnable
                    val insertPos = lastArg.textRange.endOffset
                    val missingCount = propertyCount - currentArgCount
                    
                    // Build the text to insert: ", nil, nil, ..."
                    val nilArgs = List(missingCount) { "nil" }.joinToString(", ", ", ")
                    doc.insertString(insertPos, nilArgs)
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
            
            PsiDocumentManager.getInstance(project).commitDocument(doc)
        }
        
        executeWithAppropriateWriteAction(project, file, runnable)
    }
    
    /**
     * Executes the given runnable with the appropriate write action context.
     * Handles three scenarios:
     * 1. Already in write action - runs directly
     * 2. Not in any action - wraps in WriteCommandAction
     * 3. In read action (preview) - runs without wrapping to avoid deadlock
     */
    private fun executeWithAppropriateWriteAction(project: Project, file: PsiFile, runnable: Runnable) {
        val app = ApplicationManager.getApplication()
        when {
            app.isWriteAccessAllowed -> {
                // Already in write action, just run directly
                runnable.run()
            }
            !app.isReadAccessAllowed -> {
                // Not in any action, wrap in WriteCommandAction
                WriteCommandAction.runWriteCommandAction(project, getText(), null, runnable, file)
            }
            else -> {
                // In read action (preview generation), just run without wrapping
                // The preview system will handle the write action properly
                runnable.run()
            }
        }
    }
}