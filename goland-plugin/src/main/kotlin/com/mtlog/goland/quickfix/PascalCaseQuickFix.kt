package com.mtlog.goland.quickfix

import com.intellij.codeInspection.LocalQuickFixAndIntentionActionOnPsiElement
import com.intellij.openapi.editor.Editor
import com.intellij.openapi.project.Project
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.mtlog.goland.MtlogBundle

class PascalCaseQuickFix(
    element: PsiElement?,
    private val propertyName: String
) : LocalQuickFixAndIntentionActionOnPsiElement(element) {
    
    override fun getText(): String = MtlogBundle.message("quickfix.pascalcase.name")
    
    override fun getFamilyName(): String = MtlogBundle.message("quickfix.family.name")
    
    override fun invoke(
        project: Project,
        file: PsiFile,
        editor: Editor?,
        startElement: PsiElement,
        endElement: PsiElement
    ) {
        val text = startElement.text
        val pascalCase = toPascalCase(propertyName)
        val newText = text.replace(propertyName, pascalCase)
        
        editor?.document?.replaceString(
            startElement.textRange.startOffset,
            startElement.textRange.endOffset,
            newText
        )
    }
    
    private fun toPascalCase(name: String): String {
        return name.split("_", "-", " ")
            .joinToString("") { part ->
                part.replaceFirstChar { it.uppercase() }
            }
    }
}