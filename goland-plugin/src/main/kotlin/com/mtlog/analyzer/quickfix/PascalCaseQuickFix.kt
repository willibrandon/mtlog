package com.mtlog.analyzer.quickfix

import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiDocumentManager
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile

/**
 * Quick fix to convert property names to PascalCase.
 */
class PascalCaseQuickFix(
    element: PsiElement?,
    private val propertyName: String
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {

    override fun getFamilyName() = "mtlog"
    override fun getText()      = "Convert to PascalCase"

    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        val doc = editor?.document
            ?: PsiDocumentManager.getInstance(project).getDocument(file)
            ?: return

        WriteCommandAction.runWriteCommandAction(project, text, null, Runnable {
            val litText    = startElement.text                   // `"Processing {user_id}"`
            val innerIndex = litText.indexOf(propertyName)       // position of user_id inside it
            if (innerIndex < 0) return@Runnable                  // safety

            val absStart = startElement.textRange.startOffset + innerIndex
            val absEnd   = absStart + propertyName.length

            doc.replaceString(
                absStart,
                absEnd,
                toPascalCase(propertyName)                       // "UserId"
            )
            PsiDocumentManager.getInstance(project).commitDocument(doc)
        }, file)

        FileDocumentManager.getInstance().saveDocument(doc)      // external analyzer sees change
    }

    /**
     * Converts snake_case or kebab-case to PascalCase.
     */
    private fun toPascalCase(s: String) =
        s.split('_', '-', ' ').joinToString("") { it.replaceFirstChar(Char::uppercase) }
}